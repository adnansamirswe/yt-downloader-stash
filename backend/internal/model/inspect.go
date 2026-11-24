package model

// InspectRequest is the JSON payload for POST /api/v1/inspect.
type InspectRequest struct {
	URL  string `json:"url"`
	Mode string `json:"mode,omitempty"` // "auto" | "video" | "playlist"
}

// InspectResponse unifies single video and playlist inspection outputs.
type InspectResponse struct {
	Kind        string            `json:"kind"` // "video" | "playlist"
	SourceURL   string            `json:"sourceUrl"`
	HasPlaylist bool              `json:"hasPlaylist"`
	PlaylistURL string            `json:"playlistUrl,omitempty"`
	Cached      bool              `json:"cached"`
	Video       *VideoMetadata    `json:"video,omitempty"`
	Playlist    *PlaylistMetadata `json:"playlist,omitempty"`
}

// VideoMetadata describes a single YouTube video and its curated download presets.
type VideoMetadata struct {
	VideoID         string          `json:"videoId"`
	Title           string          `json:"title"`
	Channel         string          `json:"channel"`
	DurationSeconds int64           `json:"durationSeconds"`
	ThumbnailURL    string          `json:"thumbnailUrl"`
	UploadDate      string          `json:"uploadDate,omitempty"`
	ViewCount       int64           `json:"viewCount,omitempty"`
	IsLive          bool            `json:"isLive"`
	VideoPresets    []VideoPreset   `json:"videoPresets"`
	AudioPresets    []AudioPreset   `json:"audioPresets"`
}

// VideoPreset represents a clean video download option derived from actual available streams.
type VideoPreset struct {
	ID             string `json:"id"`
	Quality        string `json:"quality"` // "2160p", "1440p", "1080p", "720p", "480p", "360p"
	Label          string `json:"label"`
	Height         int    `json:"height"`
	FPS            int    `json:"fps,omitempty"`
	Container      string `json:"container"` // "mp4" | "mkv" | "webm"
	VideoCodec     string `json:"videoCodec"`
	AudioCodec     string `json:"audioCodec"`
	FilesizeApprox int64  `json:"filesizeApprox"`
	FastRemux      bool   `json:"fastRemux"`
}

// AudioPreset represents an audio download option with native remux vs transcode clarity.
type AudioPreset struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Container      string `json:"container"` // "m4a" | "opus" | "mp3" | "flac" | "wav"
	Codec          string `json:"codec"`
	BitrateKbps    int    `json:"bitrateKbps,omitempty"`
	BitrateLabel   string `json:"bitrateLabel"`
	FilesizeApprox int64  `json:"filesizeApprox"`
	FastRemux      bool   `json:"fastRemux"`
}

// PlaylistMetadata holds fast --flat-playlist results for selective downloading.
type PlaylistMetadata struct {
	PlaylistID           string          `json:"playlistId"`
	Title                string          `json:"title"`
	Channel              string          `json:"channel"`
	ThumbnailURL         string          `json:"thumbnailUrl"`
	EntryCount           int             `json:"entryCount"`
	TotalDurationSeconds int64           `json:"totalDurationSeconds"`
	Entries              []PlaylistEntry `json:"entries"`
}

// PlaylistEntry describes a single track inside a playlist.
type PlaylistEntry struct {
	Index           int    `json:"index"`
	VideoID         string `json:"videoId"`
	Title           string `json:"title"`
	Channel         string `json:"channel"`
	DurationSeconds int64  `json:"durationSeconds"`
	ThumbnailURL    string `json:"thumbnailUrl"`
	Available       bool   `json:"available"`
}
