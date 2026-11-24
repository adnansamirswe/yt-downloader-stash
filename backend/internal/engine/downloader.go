package engine

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"stash-backend/internal/model"
	"stash-backend/internal/security"
)

// DownloadOptions defines the resolved download parameters for a single track.
type DownloadOptions struct {
	VideoURL      string
	TrackDir      string
	BaseFilename  string // Sanitized desired stem (without extension)
	MediaType     string // "video" | "audio"
	Quality       string // "best" | "2160p" | "1440p" | "1080p" | "720p" | "480p" | "360p"
	Container     string // "mp4" | "mkv" | "webm" | "m4a" | "opus" | "mp3" | "flac" | "wav"
	AudioBitrate  string // "320k" | "192k" | "128k"
	Fragments     int
	EmbedMetadata bool
}

// DownloadResult holds the verified local artifact path and display metadata.
type DownloadResult struct {
	FilePath        string
	DisplayFilename string
	SizeBytes       int64
}

// ProgressCallback receives live structured updates while a track downloads/muxes.
type ProgressCallback func(p ParsedProgress)

// DownloadTrack runs yt-dlp + FFmpeg stream-copy/transcode into an isolated directory and emits live progress.
func DownloadTrack(ctx context.Context, tooling *Tooling, opts DownloadOptions, logger *slog.Logger, onProgress ProgressCallback) (*DownloadResult, error) {
	if tooling.YtDlpPath == "" {
		return nil, errors.New("yt-dlp binary is not available")
	}

	if err := os.MkdirAll(opts.TrackDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to prepare track workspace: %w", err)
	}

	outTemplate := filepath.Join(opts.TrackDir, "media.%(ext)s")
	args := buildDownloadArgs(tooling, opts, outTemplate)

	cmd := NewGroupCommand(ctx, tooling.YtDlpPath, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var finalPath string
	var lastEmit time.Time

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for scanner.Scan() {
		line := scanner.Text()
		parsed, ok := ParseProgressLine(line)
		if !ok {
			continue
		}
		if parsed.Kind == "final" {
			finalPath = parsed.FinalPath
			continue
		}
		if onProgress != nil {
			now := time.Now()
			if parsed.Kind == "postprocess" || parsed.Percent >= 100 || now.Sub(lastEmit) >= 120*time.Millisecond {
				lastEmit = now
				onProgress(parsed)
			}
		}
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		stderrStr := stderrBuf.String()
		logger.Warn("track_download_failed", "url", opts.VideoURL, "stderr", truncateString(stderrStr, 800))
		return nil, errors.New(classifyYtDlpError(stderrStr))
	}

	// Fallback discovery inside the isolated track directory if --print did not report a path
	if finalPath == "" {
		discovered, err := discoverOutputFile(opts.TrackDir)
		if err != nil {
			return nil, errors.New("download finished but output media file could not be located")
		}
		finalPath = discovered
	}

	safePath, err := security.EnsurePathWithinDir(opts.TrackDir, finalPath)
	if err != nil {
		return nil, errors.New("output file failed workspace path containment check")
	}

	info, err := os.Stat(safePath)
	if err != nil || info.IsDir() {
		return nil, errors.New("downloaded output file is unreadable")
	}

	ext := strings.ToLower(filepath.Ext(safePath))
	if ext == "" {
		ext = "." + opts.Container
	}
	displayName := security.SanitizeFilename(opts.BaseFilename, 110) + ext

	return &DownloadResult{
		FilePath:        safePath,
		DisplayFilename: displayName,
		SizeBytes:       info.Size(),
	}, nil
}

func buildDownloadArgs(tooling *Tooling, opts DownloadOptions, outTemplate string) []string {
	frags := opts.Fragments
	if frags < 1 {
		frags = 4
	}

	args := append(tooling.BaseYtDlpArgs(),
		"--no-playlist",
		"--newline",
		"--concurrent-fragments", strconv.Itoa(frags),
		"--retries", "3",
		"--fragment-retries", "5",
		"--progress-template", "download:STASH_DL|%(progress._percent_str)s|%(progress.downloaded_bytes)s|%(progress.total_bytes)s|%(progress.total_bytes_estimate)s|%(progress.speed)s|%(progress.eta)s",
		"--progress-template", "postprocess:STASH_PP|%(progress.status)s",
		"--print", "after_move:STASH_FINAL|%(filepath)s",
		"-o", outTemplate,
	)

	if opts.MediaType == model.MediaTypeAudio {
		args = append(args, buildAudioArgs(opts)...)
	} else {
		args = append(args, buildVideoArgs(opts)...)
	}

	if opts.EmbedMetadata && opts.Container != "wav" {
		args = append(args, "--embed-metadata")
		if opts.Container == "mp4" || opts.Container == "mkv" || opts.Container == "m4a" || opts.Container == "mp3" || opts.Container == "flac" {
			args = append(args, "--embed-thumbnail")
		}
	}

	args = append(args, "--", opts.VideoURL)
	return args
}

func buildVideoArgs(opts DownloadOptions) []string {
	heightFilter := ""
	if h := parseHeightQuality(opts.Quality); h > 0 {
		heightFilter = fmt.Sprintf("[height<=%d]", h)
	}

	switch opts.Container {
	case "webm":
		selector := fmt.Sprintf("bv*%s[ext=webm]+ba[ext=webm]/bv*%s+ba/b%s", heightFilter, heightFilter, heightFilter)
		return []string{
			"-f", selector,
			"--merge-output-format", "webm",
		}
	case "mkv":
		selector := fmt.Sprintf("bv*%s+ba/b%s", heightFilter, heightFilter)
		return []string{
			"-f", selector,
			"--merge-output-format", "mkv",
		}
	default: // "mp4" — fast H.264+AAC remux with +faststart whenever available
		selector := fmt.Sprintf(
			"bv*%s[vcodec^=avc1]+ba[ext=m4a]/bv*%s[ext=mp4]+ba[ext=m4a]/bv*%s+ba/b%s",
			heightFilter, heightFilter, heightFilter, heightFilter,
		)
		return []string{
			"-f", selector,
			"--merge-output-format", "mp4",
			"--postprocessor-args", "Merger+ffmpeg_o:-movflags +faststart",
		}
	}
}

func buildAudioArgs(opts DownloadOptions) []string {
	switch opts.Container {
	case "opus":
		return []string{
			"-f", "ba[acodec=opus]/ba/b",
			"-x",
			"--audio-format", "opus",
		}
	case "mp3":
		q := "320K"
		switch strings.ToLower(opts.AudioBitrate) {
		case "192k":
			q = "192K"
		case "128k":
			q = "128K"
		}
		return []string{
			"-f", "ba/b",
			"-x",
			"--audio-format", "mp3",
			"--audio-quality", q,
			"--postprocessor-args", "ExtractAudio+ffmpeg_o:-threads 0",
		}
	case "flac":
		return []string{
			"-f", "ba/b",
			"-x",
			"--audio-format", "flac",
			"--postprocessor-args", "ExtractAudio+ffmpeg_o:-threads 0",
		}
	case "wav":
		return []string{
			"-f", "ba/b",
			"-x",
			"--audio-format", "wav",
		}
	default: // "m4a" — native AAC fast remux
		return []string{
			"-f", "ba[ext=m4a]/ba/b",
			"-x",
			"--audio-format", "m4a",
		}
	}
}

func parseHeightQuality(q string) int {
	switch strings.ToLower(strings.TrimSpace(q)) {
	case "2160p":
		return 2160
	case "1440p":
		return 1440
	case "1080p":
		return 1080
	case "720p":
		return 720
	case "480p":
		return 480
	case "360p":
		return 360
	default:
		return 0
	}
}

func discoverOutputFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var best string
	var bestSize int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".ytdl") || strings.HasSuffix(name, ".webp") || strings.HasSuffix(name, ".jpg") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() >= bestSize {
			bestSize = info.Size()
			best = filepath.Join(dir, name)
		}
	}
	if best == "" {
		return "", os.ErrNotExist
	}
	return best, nil
}
