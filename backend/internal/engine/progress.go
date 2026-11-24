package engine

import (
	"math"
	"strconv"
	"strings"
)

const (
	ProgressDownloadPrefix = "STASH_DL|"
	ProgressPostPrefix     = "STASH_PP|"
	ProgressFinalPrefix    = "STASH_FINAL|"
)

// ParsedProgress captures a single structured progress line emitted by yt-dlp.
type ParsedProgress struct {
	Kind            string // "download" | "postprocess" | "final"
	Percent         float64
	DownloadedBytes int64
	TotalBytes      int64
	SpeedBytes      float64
	ETASeconds      int64
	PostStatus      string
	FinalPath       string
}

// ParseProgressLine parses Stash's custom yt-dlp --progress-template and --print output lines.
func ParseProgressLine(line string) (ParsedProgress, bool) {
	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, ProgressFinalPrefix) {
		path := strings.TrimSpace(strings.TrimPrefix(trimmed, ProgressFinalPrefix))
		if path != "" {
			return ParsedProgress{Kind: "final", FinalPath: path}, true
		}
		return ParsedProgress{}, false
	}

	if strings.HasPrefix(trimmed, ProgressPostPrefix) {
		status := strings.TrimSpace(strings.TrimPrefix(trimmed, ProgressPostPrefix))
		return ParsedProgress{Kind: "postprocess", PostStatus: status}, true
	}

	if !strings.HasPrefix(trimmed, ProgressDownloadPrefix) {
		return ParsedProgress{}, false
	}

	payload := strings.TrimPrefix(trimmed, ProgressDownloadPrefix)
	parts := strings.Split(payload, "|")
	if len(parts) < 6 {
		return ParsedProgress{}, false
	}

	pct := parsePercent(parts[0])
	downloaded := parseInt64(parts[1])
	total := parseInt64(parts[2])
	totalEst := parseInt64(parts[3])
	if total <= 0 {
		total = totalEst
	}
	speed := parseFloat64(parts[4])
	eta := parseInt64(parts[5])

	if pct <= 0 && downloaded > 0 && total > 0 {
		pct = (float64(downloaded) / float64(total)) * 100.0
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	return ParsedProgress{
		Kind:            "download",
		Percent:         math.Round(pct*10) / 10,
		DownloadedBytes: downloaded,
		TotalBytes:      total,
		SpeedBytes:      speed,
		ETASeconds:      eta,
	}, true
}

func parsePercent(raw string) float64 {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "%"))
	return parseFloat64(s)
}

func parseInt64(raw string) int64 {
	s := strings.TrimSpace(raw)
	if s == "" || s == "NA" || s == "None" {
		return 0
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
		return int64(math.Round(f))
	}
	return 0
}

func parseFloat64(raw string) float64 {
	s := strings.TrimSpace(raw)
	if s == "" || s == "NA" || s == "None" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}
