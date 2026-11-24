package model

import "time"

const (
	MediaTypeVideo = "video"
	MediaTypeAudio = "audio"

	JobKindSingle   = "single"
	JobKindPlaylist = "playlist"

	JobStatusQueued    = "queued"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusPartial   = "partial"
	JobStatusFailed    = "failed"
	JobStatusCancelled = "cancelled"

	TrackStatusQueued      = "queued"
	TrackStatusDownloading = "downloading"
	TrackStatusMuxing      = "muxing"
	TrackStatusCompleted   = "completed"
	TrackStatusFailed      = "failed"
	TrackStatusCancelled   = "cancelled"
)

// SingleJobRequest starts a single video or audio download job.
type SingleJobRequest struct {
	URL           string `json:"url"`
	Title         string `json:"title,omitempty"`
	MediaType     string `json:"mediaType"`               // "video" | "audio"
	Quality       string `json:"quality,omitempty"`       // "best" | "2160p" | "1440p" | "1080p" | "720p" | "480p" | "360p"
	Container     string `json:"container"`               // "mp4" | "mkv" | "webm" | "m4a" | "opus" | "mp3" | "flac" | "wav"
	AudioBitrate  string `json:"audioBitrate,omitempty"`  // "320k" | "192k" | "128k"
	EmbedMetadata bool   `json:"embedMetadata,omitempty"` // Embed tags/thumbnail when supported
}

// PlaylistSelectionItem identifies a selected track in a playlist batch request.
type PlaylistSelectionItem struct {
	VideoID string `json:"videoId"`
	Title   string `json:"title"`
	Index   int    `json:"index"`
}

// PlaylistJobRequest starts a parallel selective playlist download job.
type PlaylistJobRequest struct {
	PlaylistURL   string                  `json:"playlistUrl"`
	PlaylistTitle string                  `json:"playlistTitle,omitempty"`
	Items         []PlaylistSelectionItem `json:"items"`
	MediaType     string                  `json:"mediaType"` // "video" | "audio"
	Quality       string                  `json:"quality,omitempty"`
	Container     string                  `json:"container"`
	AudioBitrate  string                  `json:"audioBitrate,omitempty"`
	Concurrency   int                     `json:"concurrency,omitempty"` // 1..6 (clamped by server config)
	EmbedMetadata bool                    `json:"embedMetadata,omitempty"`
}

// TrackState represents real-time progress for a single track inside a job.
type TrackState struct {
	Index           int     `json:"index"`
	VideoID         string  `json:"videoId"`
	Title           string  `json:"title"`
	Status          string  `json:"status"`
	Percent         float64 `json:"percent"`
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	SpeedBytes      float64 `json:"speedBytes"`
	ETASeconds      int64   `json:"etaSeconds"`
	OutputFilename  string  `json:"outputFilename,omitempty"`
	OutputSizeBytes int64   `json:"outputSizeBytes,omitempty"`
	Error           string  `json:"error,omitempty"`
}

// JobSnapshot is an immutable view of a download job safe for JSON serialization.
type JobSnapshot struct {
	JobID               string       `json:"jobId"`
	Kind                string       `json:"kind"`
	Title               string       `json:"title"`
	MediaType           string       `json:"mediaType"`
	Quality             string       `json:"quality,omitempty"`
	Container           string       `json:"container"`
	AudioBitrate        string       `json:"audioBitrate,omitempty"`
	Status              string       `json:"status"`
	CreatedAt           time.Time    `json:"createdAt"`
	CompletedAt         *time.Time   `json:"completedAt,omitempty"`
	TotalTracks         int          `json:"totalTracks"`
	CompletedTracks     int          `json:"completedTracks"`
	FailedTracks        int          `json:"failedTracks"`
	OverallPercent      float64      `json:"overallPercent"`
	AggregateSpeedBytes float64      `json:"aggregateSpeedBytes"`
	OutputFilename      string       `json:"outputFilename,omitempty"`
	OutputSizeBytes     int64        `json:"outputSizeBytes,omitempty"`
	Error               string       `json:"error,omitempty"`
	Tracks              []TrackState `json:"tracks"`
}

// ProgressEvent is emitted over Server-Sent Events (/api/v1/jobs/{id}/events).
type ProgressEvent struct {
	Type  string       `json:"type"` // "snapshot" | "update" | "done"
	Job   *JobSnapshot `json:"job"`
	Track *TrackState  `json:"track,omitempty"`
}

// HealthResponse reports backend and media binary readiness.
type HealthResponse struct {
	Status              string `json:"status"`
	YtDlpVersion        string `json:"ytDlpVersion"`
	FFmpegAvailable     bool   `json:"ffmpegAvailable"`
	FFmpegVersion       string `json:"ffmpegVersion,omitempty"`
	JSRuntime           string `json:"jsRuntime"`
	PotProviderEnabled  bool   `json:"potProviderEnabled"`
	PotProviderHealthy  bool   `json:"potProviderHealthy"`
	MaxConcurrentTracks int    `json:"maxConcurrentTracks"`
	ActiveJobs          int    `json:"activeJobs"`
}
