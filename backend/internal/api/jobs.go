package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"stash-backend/internal/engine"
	"stash-backend/internal/model"
	"stash-backend/internal/security"
)

func (s *Server) handleStartSingleJob(w http.ResponseWriter, r *http.Request) {
	if !s.allowRequest(r, s.jobLimiter) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many download requests. Please wait briefly.")
		return
	}

	var req model.SingleJobRequest
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	validated, err := security.ValidateYouTubeURL(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_url", err.Error())
		return
	}
	if validated.VideoID == "" {
		writeError(w, http.StatusBadRequest, "invalid_url", "single download requires a YouTube video URL")
		return
	}

	mt, q, c, br, err := normalizeMediaParams(req.MediaType, req.Quality, req.Container, req.AudioBitrate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_format", err.Error())
		return
	}
	req.MediaType = mt
	req.Quality = q
	req.Container = c
	req.AudioBitrate = br

	snap, err := s.jobs.StartSingleJob(validated, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "job_start_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, snap)
}

func (s *Server) handleStartPlaylistJob(w http.ResponseWriter, r *http.Request) {
	if !s.allowRequest(r, s.jobLimiter) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many download requests. Please wait briefly.")
		return
	}

	var req model.PlaylistJobRequest
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if req.PlaylistURL != "" {
		if _, err := security.ValidateYouTubeURL(req.PlaylistURL); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_url", err.Error())
			return
		}
	}

	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "empty_selection", "Select at least one track from the playlist.")
		return
	}
	if len(req.Items) > s.cfg.MaxPlaylistItems {
		writeError(w, http.StatusBadRequest, "too_many_items", fmt.Sprintf("At most %d tracks can be downloaded per batch.", s.cfg.MaxPlaylistItems))
		return
	}

	seen := make(map[string]struct{}, len(req.Items))
	deduped := make([]model.PlaylistSelectionItem, 0, len(req.Items))
	for _, it := range req.Items {
		vid := strings.TrimSpace(it.VideoID)
		if !security.ValidateVideoID(vid) {
			writeError(w, http.StatusBadRequest, "invalid_video_id", "Selection contains an invalid YouTube video identifier.")
			return
		}
		if _, exists := seen[vid]; exists {
			continue
		}
		seen[vid] = struct{}{}
		deduped = append(deduped, model.PlaylistSelectionItem{
			VideoID: vid,
			Title:   security.SanitizeFilename(it.Title, 120),
			Index:   it.Index,
		})
	}
	req.Items = deduped

	mt, q, c, br, err := normalizeMediaParams(req.MediaType, req.Quality, req.Container, req.AudioBitrate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_format", err.Error())
		return
	}
	req.MediaType = mt
	req.Quality = q
	req.Container = c
	req.AudioBitrate = br

	if req.Concurrency < 0 || req.Concurrency > 6 {
		writeError(w, http.StatusBadRequest, "invalid_concurrency", "concurrency must be between 1 and 6")
		return
	}

	snap, err := s.jobs.StartPlaylistJob(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "job_start_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, snap)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if !security.ValidateJobID(jobID) {
		writeError(w, http.StatusBadRequest, "invalid_job_id", "Invalid job identifier.")
		return
	}

	snap, err := s.jobs.GetSnapshot(jobID)
	if err != nil {
		writeJobError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if !security.ValidateJobID(jobID) {
		writeError(w, http.StatusBadRequest, "invalid_job_id", "Invalid job identifier.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "SSE streaming is unsupported.")
		return
	}

	initialSnap, ch, unsub, err := s.jobs.Subscribe(jobID)
	if err != nil {
		writeJobError(w, err)
		return
	}
	defer unsub()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if err := writeSSEEvent(w, "snapshot", model.ProgressEvent{Type: "snapshot", Job: &initialSnap}); err != nil {
		return
	}
	flusher.Flush()

	// If job already reached a terminal state (except partial, which might receive retries), close cleanly after snapshot.
	if initialSnap.Status == model.JobStatusCompleted || initialSnap.Status == model.JobStatusFailed || initialSnap.Status == model.JobStatusCancelled {
		return
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev := <-ch:
			if err := writeSSEEvent(w, ev.Type, ev); err != nil {
				return
			}
			flusher.Flush()
			if ev.Type == "done" && ev.Job != nil &&
				(ev.Job.Status == model.JobStatusCompleted || ev.Job.Status == model.JobStatusCancelled) {
				return
			}
		}
	}
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if !security.ValidateJobID(jobID) {
		writeError(w, http.StatusBadRequest, "invalid_job_id", "Invalid job identifier.")
		return
	}

	snap, err := s.jobs.CancelJob(jobID)
	if err != nil {
		writeJobError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleRetryTrack(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	videoID := r.PathValue("videoId")
	if !security.ValidateJobID(jobID) || !security.ValidateVideoID(videoID) {
		writeError(w, http.StatusBadRequest, "invalid_identifier", "Invalid job or video identifier.")
		return
	}

	snap, err := s.jobs.RetryTrack(jobID, videoID)
	if err != nil {
		writeJobError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, snap)
}

func (s *Server) handleDownloadJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if !security.ValidateJobID(jobID) {
		writeError(w, http.StatusBadRequest, "invalid_job_id", "Invalid job identifier.")
		return
	}

	snap, err := s.jobs.GetSnapshot(jobID)
	if err != nil {
		writeJobError(w, err)
		return
	}

	if snap.Kind == model.JobKindSingle {
		filePath, displayName, size, err := s.jobs.SingleFileArtifact(jobID)
		if err != nil {
			writeJobError(w, err)
			return
		}
		serveAttachmentFile(w, filePath, displayName, size)
		return
	}

	// Playlist job: stream all completed tracks directly as a zero-compression .zip archive
	zipName, entries, err := s.jobs.PlaylistZipArtifacts(jobID)
	if err != nil {
		writeJobError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", security.ContentDispositionAttachment(zipName))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if err := engine.StreamStoredZip(w, entries); err != nil {
		s.logger.Warn("playlist_zip_stream_error", "job_id", jobID, "error", err)
	}
}

func (s *Server) handleDownloadTrack(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	videoID := r.PathValue("videoId")
	if !security.ValidateJobID(jobID) || !security.ValidateVideoID(videoID) {
		writeError(w, http.StatusBadRequest, "invalid_identifier", "Invalid job or video identifier.")
		return
	}

	filePath, displayName, size, err := s.jobs.TrackArtifact(jobID, videoID)
	if err != nil {
		writeJobError(w, err)
		return
	}
	serveAttachmentFile(w, filePath, displayName, size)
}

func serveAttachmentFile(w http.ResponseWriter, filePath, displayName string, size int64) {
	f, err := os.Open(filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file_unavailable", "Download file is no longer available.")
		return
	}
	defer func() { _ = f.Close() }()

	if size <= 0 {
		if info, err := f.Stat(); err == nil {
			size = info.Size()
		}
	}

	w.Header().Set("Content-Type", contentTypeForFilename(displayName))
	w.Header().Set("Content-Disposition", security.ContentDispositionAttachment(displayName))
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	buf := make([]byte, 64*1024)
	_, _ = io.CopyBuffer(w, f, buf)
}

func contentTypeForFilename(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".webm"):
		return "video/webm"
	case strings.HasSuffix(lower, ".mkv"):
		return "video/x-matroska"
	case strings.HasSuffix(lower, ".m4a"):
		return "audio/mp4"
	case strings.HasSuffix(lower, ".opus"):
		return "audio/opus"
	case strings.HasSuffix(lower, ".mp3"):
		return "audio/mpeg"
	case strings.HasSuffix(lower, ".flac"):
		return "audio/flac"
	case strings.HasSuffix(lower, ".wav"):
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

func writeSSEEvent(w io.Writer, eventName string, payload model.ProgressEvent) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, raw)
	return err
}

func writeJobError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, engine.ErrJobNotFound), errors.Is(err, engine.ErrTrackNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, engine.ErrNotReady):
		writeError(w, http.StatusConflict, "not_ready", err.Error())
	case errors.Is(err, engine.ErrTooManyJobs):
		writeError(w, http.StatusTooManyRequests, "server_busy", err.Error())
	default:
		writeError(w, http.StatusBadRequest, "job_error", err.Error())
	}
}
