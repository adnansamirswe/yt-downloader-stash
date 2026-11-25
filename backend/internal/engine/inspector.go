package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"stash-backend/internal/cache"
	"stash-backend/internal/model"
	"stash-backend/internal/security"
)

// Inspector resolves YouTube single video and flat playlist metadata with TTL caching.
type Inspector struct {
	tooling *Tooling
	cache   *cache.TTLCache[string, *model.InspectResponse]
	logger  *slog.Logger
}

// NewInspector constructs an Inspector backed by a bounded TTL cache.
func NewInspector(tooling *Tooling, ttl time.Duration, logger *slog.Logger) *Inspector {
	return &Inspector{
		tooling: tooling,
		cache:   cache.New[string, *model.InspectResponse](ttl, 256),
		logger:  logger,
	}
}

// Inspect resolves a validated YouTube URL in auto, video, or playlist mode.
func (ins *Inspector) Inspect(ctx context.Context, validated *security.ValidatedURL, mode string) (*model.InspectResponse, error) {
	if ins.tooling.YtDlpPath == "" {
		return nil, errors.New("yt-dlp binary is not available on the server")
	}

	targetKind := "video"
	switch mode {
	case "playlist":
		if validated.PlaylistID == "" {
			return nil, errors.New("this URL does not reference a YouTube playlist")
		}
		targetKind = "playlist"
	case "video":
		if validated.VideoID == "" {
			return nil, errors.New("this URL does not reference a single YouTube video")
		}
		targetKind = "video"
	default: // "auto"
		if validated.Kind == "playlist" {
			targetKind = "playlist"
		} else {
			targetKind = "video"
		}
	}

	cacheKey := targetKind + ":"
	if targetKind == "playlist" {
		cacheKey += validated.PlaylistID
	} else {
		cacheKey += validated.VideoID
	}

	if cached, ok := ins.cache.Get(cacheKey); ok {
		copyResp := *cached
		copyResp.Cached = true
		if targetKind == "video" && validated.PlaylistURL != "" {
			copyResp.HasPlaylist = true
			copyResp.PlaylistURL = validated.PlaylistURL
		}
		return &copyResp, nil
	}

	var resp *model.InspectResponse
	var err error
	if targetKind == "playlist" {
		resp, err = ins.inspectPlaylist(ctx, validated)
	} else {
		resp, err = ins.inspectVideo(ctx, validated)
	}
	if err != nil {
		return nil, err
	}

	ins.cache.Set(cacheKey, resp)
	out := *resp
	out.Cached = false
	return &out, nil
}

type rawYtDlpVideo struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Channel    string           `json:"channel"`
	Uploader   string           `json:"uploader"`
	Duration   float64          `json:"duration"`
	Thumbnail  string           `json:"thumbnail"`
	UploadDate string           `json:"upload_date"`
	ViewCount  int64            `json:"view_count"`
	IsLive     bool             `json:"is_live"`
	Formats    []rawYtDlpFormat `json:"formats"`
}

type rawYtDlpFormat struct {
	FormatID       string  `json:"format_id"`
	Ext            string  `json:"ext"`
	VCodec         string  `json:"vcodec"`
	ACodec         string  `json:"acodec"`
	Height         int     `json:"height"`
	FPS            float64 `json:"fps"`
	Filesize       int64   `json:"filesize"`
	FilesizeApprox int64   `json:"filesize_approx"`
	TBR            float64 `json:"tbr"`
	VBR            float64 `json:"vbr"`
	ABR            float64 `json:"abr"`
}

func (ins *Inspector) inspectVideo(ctx context.Context, validated *security.ValidatedURL) (*model.InspectResponse, error) {
	inspectCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	baseArgs, cleanup := ins.tooling.PrepareYtDlpArgs()
	defer cleanup()

	args := append(baseArgs,
		"--dump-single-json",
		"--no-playlist",
		"--skip-download",
		"--",
		validated.VideoURL,
	)

	stdout, stderr, err := ins.runYtDlp(inspectCtx, args)
	if err != nil {
		ins.logger.Warn("video_inspect_failed",
			"video_id", validated.VideoID,
			"cookies_configured", ins.tooling.HasCookies(),
			"cookies_status", ins.tooling.CookiesStatus,
			"stderr", truncateString(stderr, 600),
		)
		return nil, errors.New(classifyYtDlpError(stderr))
	}

	var raw rawYtDlpVideo
	if err := json.Unmarshal(stdout, &raw); err != nil {
		ins.logger.Error("video_json_parse_failed", "error", err)
		return nil, errors.New("could not read video metadata from YouTube")
	}

	channel := strings.TrimSpace(raw.Channel)
	if channel == "" {
		channel = strings.TrimSpace(raw.Uploader)
	}
	if channel == "" {
		channel = "Unknown channel"
	}

	thumb := strings.TrimSpace(raw.Thumbnail)
	if thumb == "" && raw.ID != "" {
		thumb = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", raw.ID)
	}

	durationSec := int64(math.Round(raw.Duration))
	videoPresets, audioPresets := buildCuratedPresets(raw.Formats, durationSec)

	meta := &model.VideoMetadata{
		VideoID:         raw.ID,
		Title:           strings.TrimSpace(raw.Title),
		Channel:         channel,
		DurationSeconds: durationSec,
		ThumbnailURL:    thumb,
		UploadDate:      raw.UploadDate,
		ViewCount:       raw.ViewCount,
		IsLive:          raw.IsLive,
		VideoPresets:    videoPresets,
		AudioPresets:    audioPresets,
	}

	return &model.InspectResponse{
		Kind:        "video",
		SourceURL:   validated.VideoURL,
		HasPlaylist: validated.PlaylistURL != "",
		PlaylistURL: validated.PlaylistURL,
		Video:       meta,
	}, nil
}

type rawYtDlpPlaylist struct {
	ID         string                  `json:"id"`
	Title      string                  `json:"title"`
	Channel    string                  `json:"channel"`
	Uploader   string                  `json:"uploader"`
	Thumbnails []rawYtDlpThumbnail     `json:"thumbnails"`
	Entries    []rawYtDlpPlaylistEntry `json:"entries"`
}

type rawYtDlpThumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type rawYtDlpPlaylistEntry struct {
	ID         string              `json:"id"`
	Title      string              `json:"title"`
	Channel    string              `json:"channel"`
	Uploader   string              `json:"uploader"`
	Duration   float64             `json:"duration"`
	Thumbnails []rawYtDlpThumbnail `json:"thumbnails"`
}

func (ins *Inspector) inspectPlaylist(ctx context.Context, validated *security.ValidatedURL) (*model.InspectResponse, error) {
	inspectCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	baseArgs, cleanup := ins.tooling.PrepareYtDlpArgs()
	defer cleanup()

	// YouTube Radio/Mix (RD...) playlists paginate indefinitely; cap at 50 entries for fast loads.
	// Regular playlists are capped at 250 entries to prevent multi-minute inspection timeouts.
	playlistLimit := "250"
	if strings.HasPrefix(validated.PlaylistID, "RD") {
		playlistLimit = "50"
	}

	args := append(baseArgs,
		"--flat-playlist",
		"--yes-playlist",
		"--playlist-end", playlistLimit,
		"--dump-single-json",
		"--skip-download",
		"--",
		validated.PlaylistURL,
	)

	stdout, stderr, err := ins.runYtDlp(inspectCtx, args)
	if err != nil {
		ins.logger.Warn("playlist_inspect_failed",
			"playlist_id", validated.PlaylistID,
			"cookies_configured", ins.tooling.HasCookies(),
			"cookies_status", ins.tooling.CookiesStatus,
			"timed_out", errors.Is(inspectCtx.Err(), context.DeadlineExceeded),
			"stderr", truncateString(stderr, 600),
		)
		if errors.Is(inspectCtx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("timed out fetching playlist from YouTube; try again in a moment")
		}
		return nil, errors.New(classifyYtDlpError(stderr))
	}

	var raw rawYtDlpPlaylist
	if err := json.Unmarshal(stdout, &raw); err != nil {
		ins.logger.Error("playlist_json_parse_failed", "error", err)
		return nil, errors.New("could not read playlist metadata from YouTube")
	}

	channel := strings.TrimSpace(raw.Channel)
	if channel == "" {
		channel = strings.TrimSpace(raw.Uploader)
	}

	entries := make([]model.PlaylistEntry, 0, len(raw.Entries))
	var totalDuration int64
	var coverThumb string
	if len(raw.Thumbnails) > 0 {
		coverThumb = raw.Thumbnails[len(raw.Thumbnails)-1].URL
	}

	for i, e := range raw.Entries {
		vid := strings.TrimSpace(e.ID)
		title := strings.TrimSpace(e.Title)
		available := security.ValidateVideoID(vid) &&
			title != "" &&
			title != "[Private video]" &&
			title != "[Deleted video]"

		entryChannel := strings.TrimSpace(e.Channel)
		if entryChannel == "" {
			entryChannel = strings.TrimSpace(e.Uploader)
		}
		if entryChannel == "" {
			entryChannel = channel
		}

		dur := int64(math.Round(e.Duration))
		if dur < 0 {
			dur = 0
		}
		if available {
			totalDuration += dur
		}

		thumb := pickThumbnail(e.Thumbnails, vid)
		if coverThumb == "" && available && thumb != "" {
			coverThumb = thumb
		}
		if title == "" {
			title = "Unavailable track"
		}

		entries = append(entries, model.PlaylistEntry{
			Index:           i + 1,
			VideoID:         vid,
			Title:           title,
			Channel:         entryChannel,
			DurationSeconds: dur,
			ThumbnailURL:    thumb,
			Available:       available,
		})
	}

	playlistTitle := strings.TrimSpace(raw.Title)
	if playlistTitle == "" {
		playlistTitle = "YouTube Playlist"
	}

	meta := &model.PlaylistMetadata{
		PlaylistID:           validated.PlaylistID,
		Title:                playlistTitle,
		Channel:              channel,
		ThumbnailURL:         coverThumb,
		EntryCount:           len(entries),
		TotalDurationSeconds: totalDuration,
		Entries:              entries,
	}

	return &model.InspectResponse{
		Kind:        "playlist",
		SourceURL:   validated.PlaylistURL,
		HasPlaylist: true,
		PlaylistURL: validated.PlaylistURL,
		Playlist:    meta,
	}, nil
}

func (ins *Inspector) runYtDlp(ctx context.Context, args []string) ([]byte, string, error) {
	cmd := NewGroupCommand(ctx, ins.tooling.YtDlpPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.String(), err
}

func pickThumbnail(thumbs []rawYtDlpThumbnail, videoID string) string {
	for i := len(thumbs) - 1; i >= 0; i-- {
		u := strings.TrimSpace(thumbs[i].URL)
		if strings.HasPrefix(u, "https://i.ytimg.com/") || strings.HasPrefix(u, "https://i9.ytimg.com/") {
			return u
		}
	}
	if security.ValidateVideoID(videoID) {
		return fmt.Sprintf("https://i.ytimg.com/vi/%s/mqdefault.jpg", videoID)
	}
	return ""
}

func buildCuratedPresets(formats []rawYtDlpFormat, durationSec int64) ([]model.VideoPreset, []model.AudioPreset) {
	var bestM4ASize int64
	var bestM4AKbps float64
	var bestOpusSize int64
	var bestOpusKbps float64

	type bestByHeight struct {
		mp4  *rawYtDlpFormat
		webm *rawYtDlpFormat
		any  *rawYtDlpFormat
	}
	buckets := make(map[int]*bestByHeight)
	standardHeights := []int{2160, 1440, 1080, 720, 480, 360}

	for i := range formats {
		f := formats[i]
		vcodec := strings.ToLower(f.VCodec)
		acodec := strings.ToLower(f.ACodec)
		hasVideo := vcodec != "" && vcodec != "none"
		hasAudio := acodec != "" && acodec != "none"

		if !hasVideo && hasAudio {
			sz := estimateFormatBytes(f, durationSec)
			kbps := f.ABR
			if kbps <= 0 {
				kbps = f.TBR
			}
			if f.Ext == "m4a" || strings.HasPrefix(acodec, "mp4a") {
				if kbps > bestM4AKbps || (kbps == bestM4AKbps && sz > bestM4ASize) {
					bestM4AKbps = kbps
					bestM4ASize = sz
				}
			}
			if strings.Contains(acodec, "opus") || f.Ext == "webm" {
				if kbps > bestOpusKbps || (kbps == bestOpusKbps && sz > bestOpusSize) {
					bestOpusKbps = kbps
					bestOpusSize = sz
				}
			}
			continue
		}

		if !hasVideo || f.Height < 240 {
			continue
		}

		h := snapStandardHeight(f.Height, standardHeights)
		b, ok := buckets[h]
		if !ok {
			b = &bestByHeight{}
			buckets[h] = b
		}

		if b.any == nil || compareVideoFormat(&f, b.any) > 0 {
			cp := f
			b.any = &cp
		}
		if strings.HasPrefix(vcodec, "avc1") || f.Ext == "mp4" {
			if b.mp4 == nil || compareVideoFormat(&f, b.mp4) > 0 {
				cp := f
				b.mp4 = &cp
			}
		}
		if strings.HasPrefix(vcodec, "vp9") || strings.HasPrefix(vcodec, "vp09") || strings.HasPrefix(vcodec, "av01") || f.Ext == "webm" {
			if b.webm == nil || compareVideoFormat(&f, b.webm) > 0 {
				cp := f
				b.webm = &cp
			}
		}
	}

	var heights []int
	for h := range buckets {
		heights = append(heights, h)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(heights)))

	videoPresets := make([]model.VideoPreset, 0, len(heights))
	for _, h := range heights {
		b := buckets[h]
		chosen := b.mp4
		container := "mp4"
		audioBytes := bestM4ASize
		audioCodec := "AAC"

		// For 1440p/4K, prefer VP9/AV1 in MKV if no H.264 stream exists at that resolution
		if chosen == nil || (h >= 1440 && !strings.HasPrefix(strings.ToLower(chosen.VCodec), "avc1")) {
			if b.webm != nil {
				chosen = b.webm
				container = "mkv"
				audioBytes = bestOpusSize
				audioCodec = "Opus"
			} else if chosen == nil {
				chosen = b.any
				container = "mkv"
				audioBytes = bestOpusSize
				audioCodec = "Opus"
			}
		}
		if chosen == nil {
			continue
		}

		videoBytes := estimateFormatBytes(*chosen, durationSec)
		if chosen.ACodec != "" && chosen.ACodec != "none" {
			audioBytes = 0
		}
		totalBytes := videoBytes + audioBytes

		quality := fmt.Sprintf("%dp", h)
		label := qualityLabel(h, int(math.Round(chosen.FPS)))
		videoPresets = append(videoPresets, model.VideoPreset{
			ID:             fmt.Sprintf("video-%dp-%s", h, container),
			Quality:        quality,
			Label:          label,
			Height:         h,
			FPS:            int(math.Round(chosen.FPS)),
			Container:      container,
			VideoCodec:     shortCodecName(chosen.VCodec),
			AudioCodec:     audioCodec,
			FilesizeApprox: totalBytes,
			FastRemux:      true,
		})
	}

	// Fallback preset if YouTube returned an unusual format list
	if len(videoPresets) == 0 {
		videoPresets = append(videoPresets, model.VideoPreset{
			ID:         "video-best-mp4",
			Quality:    "best",
			Label:      "Best Available",
			Height:     1080,
			Container:  "mp4",
			VideoCodec: "Auto",
			AudioCodec: "AAC",
			FastRemux:  true,
		})
	}

	m4aKbps := int(math.Round(bestM4AKbps))
	if m4aKbps <= 0 {
		m4aKbps = 128
	}
	opusKbps := int(math.Round(bestOpusKbps))
	if opusKbps <= 0 {
		opusKbps = 160
	}

	audioPresets := []model.AudioPreset{
		{
			ID:             "audio-m4a",
			Label:          "M4A · Native AAC",
			Container:      "m4a",
			Codec:          "AAC",
			BitrateKbps:    m4aKbps,
			BitrateLabel:   fmt.Sprintf("~%d kbps", m4aKbps),
			FilesizeApprox: bestM4ASize,
			FastRemux:      true,
		},
		{
			ID:             "audio-opus",
			Label:          "Opus · Native WebM",
			Container:      "opus",
			Codec:          "Opus",
			BitrateKbps:    opusKbps,
			BitrateLabel:   fmt.Sprintf("~%d kbps", opusKbps),
			FilesizeApprox: bestOpusSize,
			FastRemux:      true,
		},
		{
			ID:             "audio-mp3-320",
			Label:          "MP3 · High Quality",
			Container:      "mp3",
			Codec:          "MP3",
			BitrateKbps:    320,
			BitrateLabel:   "320 kbps",
			FilesizeApprox: estimateBitrateBytes(320, durationSec),
			FastRemux:      false,
		},
		{
			ID:             "audio-mp3-192",
			Label:          "MP3 · Standard",
			Container:      "mp3",
			Codec:          "MP3",
			BitrateKbps:    192,
			BitrateLabel:   "192 kbps",
			FilesizeApprox: estimateBitrateBytes(192, durationSec),
			FastRemux:      false,
		},
		{
			ID:             "audio-flac",
			Label:          "FLAC · Lossless",
			Container:      "flac",
			Codec:          "FLAC",
			BitrateLabel:   "Lossless",
			FilesizeApprox: estimateBitrateBytes(900, durationSec),
			FastRemux:      false,
		},
		{
			ID:             "audio-wav",
			Label:          "WAV · Uncompressed PCM",
			Container:      "wav",
			Codec:          "PCM",
			BitrateLabel:   "1411 kbps",
			FilesizeApprox: estimateBitrateBytes(1411, durationSec),
			FastRemux:      false,
		},
	}

	return videoPresets, audioPresets
}

func snapStandardHeight(h int, standards []int) int {
	for _, s := range standards {
		if h >= s-60 {
			return s
		}
	}
	return 360
}

func compareVideoFormat(a, b *rawYtDlpFormat) int {
	if a.FPS != b.FPS {
		if a.FPS > b.FPS {
			return 1
		}
		return -1
	}
	if a.TBR > b.TBR {
		return 1
	}
	if a.TBR < b.TBR {
		return -1
	}
	return 0
}

func estimateFormatBytes(f rawYtDlpFormat, durationSec int64) int64 {
	if f.Filesize > 0 {
		return f.Filesize
	}
	if f.FilesizeApprox > 0 {
		return f.FilesizeApprox
	}
	rateKbps := f.TBR
	if rateKbps <= 0 {
		rateKbps = f.VBR + f.ABR
	}
	return estimateBitrateBytes(rateKbps, durationSec)
}

func estimateBitrateBytes(kbps float64, durationSec int64) int64 {
	if kbps <= 0 || durationSec <= 0 {
		return 0
	}
	return int64((kbps * 1000.0 / 8.0) * float64(durationSec))
}

func qualityLabel(height, fps int) string {
	base := fmt.Sprintf("%dp", height)
	switch height {
	case 2160:
		base = "2160p · 4K"
	case 1440:
		base = "1440p · 2K"
	case 1080:
		base = "1080p · Full HD"
	case 720:
		base = "720p · HD"
	}
	if fps >= 50 {
		return fmt.Sprintf("%s %dfps", base, fps)
	}
	return base
}

func shortCodecName(raw string) string {
	lc := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lc, "avc1"):
		return "H.264"
	case strings.HasPrefix(lc, "av01"):
		return "AV1"
	case strings.HasPrefix(lc, "vp09"), strings.HasPrefix(lc, "vp9"):
		return "VP9"
	case strings.HasPrefix(lc, "hev1"), strings.HasPrefix(lc, "hvc1"):
		return "HEVC"
	default:
		return "Video"
	}
}

func classifyYtDlpError(stderr string) string {
	lc := strings.ToLower(stderr)
	switch {
	case strings.Contains(lc, "sign in to confirm you're not a bot"), strings.Contains(lc, "sign in to confirm your age"):
		return "YouTube requested bot or age verification for this video. Configure PO-token provider or cookies on the server."
	case strings.Contains(lc, "private video"):
		return "This video is private and cannot be accessed."
	case strings.Contains(lc, "video unavailable"), strings.Contains(lc, "this video is no longer available"):
		return "This video is unavailable or has been removed."
	case strings.Contains(lc, "this live event"), strings.Contains(lc, "premieres in"):
		return "Upcoming premieres and active livestreams cannot be downloaded yet."
	case strings.Contains(lc, "this playlist type is unviewable"):
		return "This YouTube Mix/Radio playlist requires a seed video URL (youtube.com/watch?v=...&list=...) to inspect."
	case strings.Contains(lc, "http error 429"), strings.Contains(lc, "too many requests"):
		return "YouTube temporarily rate-limited requests from this server. Try again shortly."
	case strings.Contains(lc, "http error 403"):
		return "YouTube rejected the media request (HTTP 403). Updating yt-dlp or enabling the PO-token provider resolves this."
	default:
		return "Could not extract media from YouTube. Verify the URL is public and try again."
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
