package engine

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"stash-backend/internal/config"
	"stash-backend/internal/model"
	"stash-backend/internal/security"
)

var (
	ErrJobNotFound   = errors.New("download job not found")
	ErrTrackNotFound = errors.New("track not found in job")
	ErrNotReady      = errors.New("download is not ready yet")
	ErrTooManyJobs   = errors.New("server is busy with active downloads; try again shortly")
)

type trackRecord struct {
	state    model.TrackState
	filePath string
	trackDir string
}

type jobRecord struct {
	mu            sync.Mutex
	id            string
	kind          string
	title         string
	mediaType     string
	quality       string
	container     string
	audioBitrate  string
	embedMetadata bool
	concurrency   int
	status        string
	createdAt     time.Time
	completedAt   *time.Time
	jobDir        string
	errMsg        string
	tracks        []*trackRecord

	ctx    context.Context
	cancel context.CancelFunc

	subMu       sync.Mutex
	subscribers map[chan model.ProgressEvent]struct{}
}

// JobManager orchestrates single and parallel playlist download jobs, SSE telemetry, cancellation, and temp cleanup.
type JobManager struct {
	cfg      config.Config
	tooling  *Tooling
	logger   *slog.Logger
	rootCtx  context.Context
	jobSem   chan struct{}

	mu   sync.RWMutex
	jobs map[string]*jobRecord
}

// NewJobManager constructs a JobManager with bounded global job concurrency and temp workspace setup.
func NewJobManager(rootCtx context.Context, cfg config.Config, tooling *Tooling, logger *slog.Logger) (*JobManager, error) {
	if err := os.MkdirAll(cfg.TempDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	m := &JobManager{
		cfg:     cfg,
		tooling: tooling,
		logger:  logger,
		rootCtx: rootCtx,
		jobSem:  make(chan struct{}, cfg.MaxConcurrentJobs),
		jobs:    make(map[string]*jobRecord),
	}

	go m.reaperLoop(rootCtx)
	return m, nil
}

// ActiveJobCount returns the number of currently queued or running jobs.
func (m *JobManager) ActiveJobCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, j := range m.jobs {
		j.mu.Lock()
		if j.status == model.JobStatusQueued || j.status == model.JobStatusRunning {
			count++
		}
		j.mu.Unlock()
	}
	return count
}

// StartSingleJob creates and launches a single video/audio download job.
func (m *JobManager) StartSingleJob(validated *security.ValidatedURL, req model.SingleJobRequest) (*model.JobSnapshot, error) {
	if validated.VideoID == "" {
		return nil, errors.New("a single video URL is required")
	}

	id := newUUIDv4()
	jobDir := filepath.Join(m.cfg.TempDir, id)
	if err := os.MkdirAll(jobDir, 0o700); err != nil {
		return nil, errors.New("could not initialize download workspace")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "YouTube " + validated.VideoID
	}

	ctx, cancel := context.WithCancel(m.rootCtx)
	tr := &trackRecord{
		state: model.TrackState{
			Index:   1,
			VideoID: validated.VideoID,
			Title:   title,
			Status:  model.TrackStatusQueued,
		},
		trackDir: filepath.Join(jobDir, "001_"+validated.VideoID),
	}

	rec := &jobRecord{
		id:            id,
		kind:          model.JobKindSingle,
		title:         title,
		mediaType:     req.MediaType,
		quality:       req.Quality,
		container:     req.Container,
		audioBitrate:  req.AudioBitrate,
		embedMetadata: req.EmbedMetadata,
		concurrency:   1,
		status:        model.JobStatusQueued,
		createdAt:     time.Now(),
		jobDir:        jobDir,
		tracks:        []*trackRecord{tr},
		ctx:           ctx,
		cancel:        cancel,
		subscribers:   make(map[chan model.ProgressEvent]struct{}),
	}

	m.mu.Lock()
	m.jobs[id] = rec
	m.mu.Unlock()

	snap := rec.snapshotLocked(false)
	go m.runJob(rec)
	return &snap, nil
}

// StartPlaylistJob creates and launches a parallel selective playlist download job.
func (m *JobManager) StartPlaylistJob(req model.PlaylistJobRequest) (*model.JobSnapshot, error) {
	id := newUUIDv4()
	jobDir := filepath.Join(m.cfg.TempDir, id)
	if err := os.MkdirAll(jobDir, 0o700); err != nil {
		return nil, errors.New("could not initialize playlist workspace")
	}

	title := strings.TrimSpace(req.PlaylistTitle)
	if title == "" {
		title = "Stash Playlist"
	}

	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = m.cfg.MaxConcurrentTracks
	}
	if concurrency > m.cfg.MaxConcurrentTracks {
		concurrency = m.cfg.MaxConcurrentTracks
	}
	if concurrency > len(req.Items) {
		concurrency = len(req.Items)
	}

	tracks := make([]*trackRecord, 0, len(req.Items))
	for i, item := range req.Items {
		idx := item.Index
		if idx <= 0 {
			idx = i + 1
		}
		trackTitle := strings.TrimSpace(item.Title)
		if trackTitle == "" {
			trackTitle = item.VideoID
		}
		tracks = append(tracks, &trackRecord{
			state: model.TrackState{
				Index:   idx,
				VideoID: item.VideoID,
				Title:   trackTitle,
				Status:  model.TrackStatusQueued,
			},
			trackDir: filepath.Join(jobDir, fmt.Sprintf("%03d_%s", i+1, item.VideoID)),
		})
	}

	ctx, cancel := context.WithCancel(m.rootCtx)
	rec := &jobRecord{
		id:            id,
		kind:          model.JobKindPlaylist,
		title:         title,
		mediaType:     req.MediaType,
		quality:       req.Quality,
		container:     req.Container,
		audioBitrate:  req.AudioBitrate,
		embedMetadata: req.EmbedMetadata,
		concurrency:   concurrency,
		status:        model.JobStatusQueued,
		createdAt:     time.Now(),
		jobDir:        jobDir,
		tracks:        tracks,
		ctx:           ctx,
		cancel:        cancel,
		subscribers:   make(map[chan model.ProgressEvent]struct{}),
	}

	m.mu.Lock()
	m.jobs[id] = rec
	m.mu.Unlock()

	snap := rec.snapshotLocked(false)
	go m.runJob(rec)
	return &snap, nil
}

// GetSnapshot returns the current state of a job.
func (m *JobManager) GetSnapshot(jobID string) (*model.JobSnapshot, error) {
	rec, ok := m.getJob(jobID)
	if !ok {
		return nil, ErrJobNotFound
	}
	snap := rec.snapshotLocked(false)
	return &snap, nil
}

// Subscribe attaches an SSE listener and immediately returns the current snapshot plus unsubscribe handle.
func (m *JobManager) Subscribe(jobID string) (model.JobSnapshot, <-chan model.ProgressEvent, func(), error) {
	rec, ok := m.getJob(jobID)
	if !ok {
		return model.JobSnapshot{}, nil, nil, ErrJobNotFound
	}

	ch := make(chan model.ProgressEvent, 32)
	rec.subMu.Lock()
	rec.subscribers[ch] = struct{}{}
	rec.subMu.Unlock()

	snap := rec.snapshotLocked(false)
	unsub := func() {
		rec.subMu.Lock()
		delete(rec.subscribers, ch)
		rec.subMu.Unlock()
	}

	return snap, ch, unsub, nil
}

// CancelJob terminates active downloads for a job and removes its temp workspace.
func (m *JobManager) CancelJob(jobID string) (*model.JobSnapshot, error) {
	rec, ok := m.getJob(jobID)
	if !ok {
		return nil, ErrJobNotFound
	}

	rec.cancel()

	rec.mu.Lock()
	now := time.Now()
	rec.status = model.JobStatusCancelled
	rec.completedAt = &now
	for _, tr := range rec.tracks {
		if tr.state.Status == model.TrackStatusQueued || tr.state.Status == model.TrackStatusDownloading || tr.state.Status == model.TrackStatusMuxing {
			tr.state.Status = model.TrackStatusCancelled
			tr.state.SpeedBytes = 0
			tr.state.ETASeconds = 0
		}
	}
	snap := rec.snapshotLocked(true)
	rec.mu.Unlock()

	rec.broadcast(model.ProgressEvent{Type: "done", Job: &snap})

	go func(dir string) {
		time.Sleep(500 * time.Millisecond)
		_ = os.RemoveAll(dir)
	}(rec.jobDir)

	return &snap, nil
}

// RetryTrack re-runs a single failed track inside an existing playlist job without affecting completed tracks.
func (m *JobManager) RetryTrack(jobID, videoID string) (*model.JobSnapshot, error) {
	rec, ok := m.getJob(jobID)
	if !ok {
		return nil, ErrJobNotFound
	}

	rec.mu.Lock()
	if rec.status == model.JobStatusCancelled {
		rec.mu.Unlock()
		return nil, errors.New("cancelled jobs cannot be retried")
	}

	var target *trackRecord
	for _, tr := range rec.tracks {
		if tr.state.VideoID == videoID {
			target = tr
			break
		}
	}
	if target == nil {
		rec.mu.Unlock()
		return nil, ErrTrackNotFound
	}
	if target.state.Status != model.TrackStatusFailed {
		rec.mu.Unlock()
		return nil, errors.New("only failed tracks can be retried")
	}

	target.state.Status = model.TrackStatusQueued
	target.state.Percent = 0
	target.state.DownloadedBytes = 0
	target.state.SpeedBytes = 0
	target.state.ETASeconds = 0
	target.state.Error = ""
	rec.status = model.JobStatusRunning
	rec.completedAt = nil
	snap := rec.snapshotLocked(true)
	rec.mu.Unlock()

	rec.broadcast(model.ProgressEvent{Type: "update", Job: &snap})

	go func() {
		m.executeTrack(rec, target, m.fragmentsForJob(rec))
		m.finalizeJobState(rec)
	}()

	return &snap, nil
}

// SingleFileArtifact returns the verified local file path and display name for a single-track job.
func (m *JobManager) SingleFileArtifact(jobID string) (filePath, displayName string, size int64, err error) {
	rec, ok := m.getJob(jobID)
	if !ok {
		return "", "", 0, ErrJobNotFound
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.tracks) == 0 || rec.tracks[0].state.Status != model.TrackStatusCompleted || rec.tracks[0].filePath == "" {
		return "", "", 0, ErrNotReady
	}
	safe, err := security.EnsurePathWithinDir(rec.jobDir, rec.tracks[0].filePath)
	if err != nil {
		return "", "", 0, ErrNotReady
	}
	return safe, rec.tracks[0].state.OutputFilename, rec.tracks[0].state.OutputSizeBytes, nil
}

// TrackArtifact returns the verified local file path and display name for one completed track in a job.
func (m *JobManager) TrackArtifact(jobID, videoID string) (filePath, displayName string, size int64, err error) {
	rec, ok := m.getJob(jobID)
	if !ok {
		return "", "", 0, ErrJobNotFound
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	for _, tr := range rec.tracks {
		if tr.state.VideoID == videoID {
			if tr.state.Status != model.TrackStatusCompleted || tr.filePath == "" {
				return "", "", 0, ErrNotReady
			}
			safe, err := security.EnsurePathWithinDir(rec.jobDir, tr.filePath)
			if err != nil {
				return "", "", 0, ErrNotReady
			}
			return safe, tr.state.OutputFilename, tr.state.OutputSizeBytes, nil
		}
	}
	return "", "", 0, ErrTrackNotFound
}

// PlaylistZipArtifacts gathers all completed tracks in a playlist job for zero-compression streaming ZIP delivery.
func (m *JobManager) PlaylistZipArtifacts(jobID string) (zipFilename string, entries []ZipEntry, err error) {
	rec, ok := m.getJob(jobID)
	if !ok {
		return "", nil, ErrJobNotFound
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	for i, tr := range rec.tracks {
		if tr.state.Status != model.TrackStatusCompleted || tr.filePath == "" {
			continue
		}
		archiveName := fmt.Sprintf("%02d - %s", i+1, tr.state.OutputFilename)
		entries = append(entries, ZipEntry{
			BaseDir:     rec.jobDir,
			FilePath:    tr.filePath,
			ArchiveName: archiveName,
		})
	}

	if len(entries) == 0 {
		return "", nil, ErrNotReady
	}

	zipFilename = security.SanitizeFilename(rec.title, 90) + ".zip"
	return zipFilename, entries, nil
}

func (m *JobManager) runJob(rec *jobRecord) {
	// Acquire a global job slot without blocking cancellation
	select {
	case m.jobSem <- struct{}{}:
		defer func() { <-m.jobSem }()
	case <-rec.ctx.Done():
		return
	}

	rec.mu.Lock()
	if rec.status == model.JobStatusCancelled {
		rec.mu.Unlock()
		return
	}
	rec.status = model.JobStatusRunning
	snap := rec.snapshotLocked(true)
	rec.mu.Unlock()
	rec.broadcast(model.ProgressEvent{Type: "update", Job: &snap})

	fragments := m.fragmentsForJob(rec)
	workerLimit := rec.concurrency
	if workerLimit < 1 {
		workerLimit = 1
	}

	trackSem := make(chan struct{}, workerLimit)
	var wg sync.WaitGroup

	for _, tr := range rec.tracks {
		if rec.ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(track *trackRecord) {
			defer wg.Done()
			select {
			case trackSem <- struct{}{}:
				defer func() { <-trackSem }()
			case <-rec.ctx.Done():
				return
			}
			m.executeTrack(rec, track, fragments)
		}(tr)
	}

	wg.Wait()
	m.finalizeJobState(rec)
}

func (m *JobManager) executeTrack(rec *jobRecord, tr *trackRecord, fragments int) {
	videoURL, err := security.CanonicalVideoURL(tr.state.VideoID)
	if err != nil {
		m.markTrackFailed(rec, tr, err.Error())
		return
	}

	rec.mu.Lock()
	tr.state.Status = model.TrackStatusDownloading
	tr.state.Error = ""
	snap := rec.snapshotLocked(true)
	trCopy := tr.state
	rec.mu.Unlock()
	rec.broadcast(model.ProgressEvent{Type: "update", Job: &snap, Track: &trCopy})

	res, err := DownloadTrack(rec.ctx, m.tooling, DownloadOptions{
		VideoURL:      videoURL,
		TrackDir:      tr.trackDir,
		BaseFilename:  tr.state.Title,
		MediaType:     rec.mediaType,
		Quality:       rec.quality,
		Container:     rec.container,
		AudioBitrate:  rec.audioBitrate,
		Fragments:     fragments,
		EmbedMetadata: rec.embedMetadata,
	}, m.logger, func(p ParsedProgress) {
		rec.mu.Lock()
		if p.Kind == "postprocess" {
			tr.state.Status = model.TrackStatusMuxing
			if tr.state.Percent < 99 {
				tr.state.Percent = 99
			}
			tr.state.SpeedBytes = 0
			tr.state.ETASeconds = 0
		} else if p.Kind == "download" {
			tr.state.Status = model.TrackStatusDownloading
			tr.state.Percent = p.Percent
			tr.state.DownloadedBytes = p.DownloadedBytes
			tr.state.TotalBytes = p.TotalBytes
			tr.state.SpeedBytes = p.SpeedBytes
			tr.state.ETASeconds = p.ETASeconds
		}
		snap := rec.snapshotLocked(true)
		trCopy := tr.state
		rec.mu.Unlock()
		rec.broadcast(model.ProgressEvent{Type: "update", Job: &snap, Track: &trCopy})
	})

	if err != nil {
		if rec.ctx.Err() != nil {
			return
		}
		m.markTrackFailed(rec, tr, err.Error())
		return
	}

	rec.mu.Lock()
	tr.state.Status = model.TrackStatusCompleted
	tr.state.Percent = 100
	tr.state.SpeedBytes = 0
	tr.state.ETASeconds = 0
	tr.state.OutputFilename = res.DisplayFilename
	tr.state.OutputSizeBytes = res.SizeBytes
	if tr.state.TotalBytes <= 0 {
		tr.state.TotalBytes = res.SizeBytes
		tr.state.DownloadedBytes = res.SizeBytes
	}
	tr.filePath = res.FilePath
	snap = rec.snapshotLocked(true)
	trCopy = tr.state
	rec.mu.Unlock()
	rec.broadcast(model.ProgressEvent{Type: "update", Job: &snap, Track: &trCopy})
}

func (m *JobManager) markTrackFailed(rec *jobRecord, tr *trackRecord, msg string) {
	rec.mu.Lock()
	tr.state.Status = model.TrackStatusFailed
	tr.state.SpeedBytes = 0
	tr.state.ETASeconds = 0
	tr.state.Error = msg
	snap := rec.snapshotLocked(true)
	trCopy := tr.state
	rec.mu.Unlock()
	rec.broadcast(model.ProgressEvent{Type: "update", Job: &snap, Track: &trCopy})
}

func (m *JobManager) finalizeJobState(rec *jobRecord) {
	rec.mu.Lock()
	if rec.status == model.JobStatusCancelled {
		rec.mu.Unlock()
		return
	}

	var completed, failed, active int
	for _, tr := range rec.tracks {
		switch tr.state.Status {
		case model.TrackStatusCompleted:
			completed++
		case model.TrackStatusFailed:
			failed++
		case model.TrackStatusQueued, model.TrackStatusDownloading, model.TrackStatusMuxing:
			active++
		}
	}

	if active > 0 {
		rec.mu.Unlock()
		return
	}

	now := time.Now()
	rec.completedAt = &now

	switch {
	case completed == len(rec.tracks):
		rec.status = model.JobStatusCompleted
	case completed > 0 && failed > 0:
		rec.status = model.JobStatusPartial
	default:
		rec.status = model.JobStatusFailed
		if len(rec.tracks) == 1 && rec.tracks[0].state.Error != "" {
			rec.errMsg = rec.tracks[0].state.Error
		} else {
			rec.errMsg = "All selected tracks failed to download."
		}
	}

	snap := rec.snapshotLocked(true)
	rec.mu.Unlock()
	rec.broadcast(model.ProgressEvent{Type: "done", Job: &snap})
}

func (m *JobManager) fragmentsForJob(rec *jobRecord) int {
	if rec.kind == model.JobKindPlaylist {
		return m.cfg.FragmentsPlaylist
	}
	return m.cfg.FragmentsSingle
}

func (m *JobManager) getJob(jobID string) (*jobRecord, bool) {
	m.mu.RLock()
	rec, ok := m.jobs[jobID]
	m.mu.RUnlock()
	return rec, ok
}

func (m *JobManager) reaperLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.cleanupAll()
			return
		case <-ticker.C:
			m.reapExpired()
		}
	}
}

func (m *JobManager) reapExpired() {
	cutoff := time.Now().Add(-m.cfg.JobTTL)

	m.mu.Lock()
	var expired []*jobRecord
	for id, rec := range m.jobs {
		rec.mu.Lock()
		isExpired := rec.completedAt != nil && rec.completedAt.Before(cutoff)
		rec.mu.Unlock()
		if isExpired {
			delete(m.jobs, id)
			expired = append(expired, rec)
		}
	}
	m.mu.Unlock()

	for _, rec := range expired {
		rec.cancel()
		_ = os.RemoveAll(rec.jobDir)
	}
}

func (m *JobManager) cleanupAll() {
	m.mu.Lock()
	all := make([]*jobRecord, 0, len(m.jobs))
	for _, rec := range m.jobs {
		all = append(all, rec)
	}
	m.mu.Unlock()

	for _, rec := range all {
		rec.cancel()
		_ = os.RemoveAll(rec.jobDir)
	}
}

func (j *jobRecord) snapshotLocked(alreadyLocked bool) model.JobSnapshot {
	if !alreadyLocked {
		j.mu.Lock()
		defer j.mu.Unlock()
	}

	tracks := make([]model.TrackState, len(j.tracks))
	var sumPercent float64
	var aggSpeed float64
	var completed, failed int
	var totalOutputSize int64

	for i, tr := range j.tracks {
		tracks[i] = tr.state
		sumPercent += tr.state.Percent
		aggSpeed += tr.state.SpeedBytes
		if tr.state.Status == model.TrackStatusCompleted {
			completed++
			totalOutputSize += tr.state.OutputSizeBytes
		} else if tr.state.Status == model.TrackStatusFailed {
			failed++
		}
	}

	overall := 0.0
	if len(j.tracks) > 0 {
		overall = math.Round((sumPercent/float64(len(j.tracks)))*10) / 10
	}

	var outputFilename string
	if j.kind == model.JobKindSingle && len(j.tracks) == 1 && j.tracks[0].state.OutputFilename != "" {
		outputFilename = j.tracks[0].state.OutputFilename
	} else if j.kind == model.JobKindPlaylist && completed > 0 {
		outputFilename = security.SanitizeFilename(j.title, 90) + ".zip"
	}

	return model.JobSnapshot{
		JobID:               j.id,
		Kind:                j.kind,
		Title:               j.title,
		MediaType:           j.mediaType,
		Quality:             j.quality,
		Container:           j.container,
		AudioBitrate:        j.audioBitrate,
		Status:              j.status,
		CreatedAt:           j.createdAt,
		CompletedAt:         j.completedAt,
		TotalTracks:         len(j.tracks),
		CompletedTracks:     completed,
		FailedTracks:        failed,
		OverallPercent:      overall,
		AggregateSpeedBytes: aggSpeed,
		OutputFilename:      outputFilename,
		OutputSizeBytes:     totalOutputSize,
		Error:               j.errMsg,
		Tracks:              tracks,
	}
}

func (j *jobRecord) broadcast(ev model.ProgressEvent) {
	j.subMu.Lock()
	defer j.subMu.Unlock()

	for ch := range j.subscribers {
		select {
		case ch <- ev:
		default:
			// Drop stale frame if a slow client lags behind; next tick restores full snapshot.
		}
	}
}

func newUUIDv4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
