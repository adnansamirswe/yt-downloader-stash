package security

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const maxURLLength = 2048

var (
	allowedHosts = map[string]struct{}{
		"youtube.com":       {},
		"www.youtube.com":   {},
		"m.youtube.com":     {},
		"music.youtube.com": {},
		"youtu.be":          {},
	}

	videoIDRegex    = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	playlistIDRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{10,128}$`)
	formatIDRegex   = regexp.MustCompile(`^[A-Za-z0-9+._-]{1,40}$`)
	jobIDRegex      = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
)

// ValidatedURL contains the parsed and strictly allowlisted YouTube URL details.
type ValidatedURL struct {
	NormalizedURL string
	VideoURL      string
	PlaylistURL   string
	VideoID       string
	PlaylistID    string
	Kind          string // "video", "playlist", or "hybrid"
}

// ValidateYouTubeURL enforces strict scheme, host allowlist, and YouTube ID validation to block SSRF.
func ValidateYouTubeURL(raw string) (*ValidatedURL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("URL is required")
	}
	if len(trimmed) > maxURLLength {
		return nil, errors.New("URL exceeds maximum allowed length")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return nil, errors.New("URL contains invalid whitespace or control characters")
		}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, errors.New("URL could not be parsed")
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && scheme != "http" {
		return nil, errors.New("only http and https YouTube URLs are supported")
	}
	if parsed.User != nil {
		return nil, errors.New("URLs with embedded credentials are not allowed")
	}
	if parsed.Port() != "" {
		return nil, errors.New("custom ports are not allowed on YouTube URLs")
	}

	host := strings.ToLower(parsed.Hostname())
	if _, ok := allowedHosts[host]; !ok {
		return nil, errors.New("only official YouTube domains (youtube.com, youtu.be, music.youtube.com) are supported")
	}

	var videoID string
	var playlistID string

	query := parsed.Query()
	if listParam := strings.TrimSpace(query.Get("list")); listParam != "" {
		if !playlistIDRegex.MatchString(listParam) {
			return nil, errors.New("invalid YouTube playlist identifier")
		}
		playlistID = listParam
	}

	pathParts := splitCleanPath(parsed.Path)
	if host == "youtu.be" {
		if len(pathParts) >= 1 && videoIDRegex.MatchString(pathParts[0]) {
			videoID = pathParts[0]
		}
	} else {
		switch {
		case len(pathParts) == 1 && pathParts[0] == "watch":
			v := strings.TrimSpace(query.Get("v"))
			if v != "" {
				if !videoIDRegex.MatchString(v) {
					return nil, errors.New("invalid YouTube video identifier")
				}
				videoID = v
			}
		case len(pathParts) >= 2 && (pathParts[0] == "shorts" || pathParts[0] == "live" || pathParts[0] == "embed" || pathParts[0] == "v"):
			if !videoIDRegex.MatchString(pathParts[1]) {
				return nil, errors.New("invalid YouTube video identifier")
			}
			videoID = pathParts[1]
		case len(pathParts) == 1 && pathParts[0] == "playlist":
			if playlistID == "" {
				return nil, errors.New("playlist URL is missing a valid list parameter")
			}
		default:
			if playlistID == "" {
				return nil, errors.New("URL must point to a YouTube video or playlist")
			}
		}
	}

	if videoID == "" && playlistID == "" {
		return nil, errors.New("URL must contain a valid YouTube video or playlist ID")
	}

	res := &ValidatedURL{
		VideoID:    videoID,
		PlaylistID: playlistID,
	}

	if videoID != "" {
		res.VideoURL = "https://www.youtube.com/watch?v=" + videoID
	}
	if playlistID != "" {
		res.PlaylistURL = "https://www.youtube.com/playlist?list=" + playlistID
	}

	switch {
	case videoID != "" && playlistID != "":
		res.Kind = "hybrid"
		res.NormalizedURL = res.VideoURL + "&list=" + playlistID
	case playlistID != "":
		res.Kind = "playlist"
		res.NormalizedURL = res.PlaylistURL
	default:
		res.Kind = "video"
		res.NormalizedURL = res.VideoURL
	}

	return res, nil
}

// ValidateVideoID verifies a strict 11-character YouTube video ID.
func ValidateVideoID(id string) bool {
	return videoIDRegex.MatchString(id)
}

// ValidateFormatID verifies a yt-dlp format selector identifier.
func ValidateFormatID(id string) bool {
	return formatIDRegex.MatchString(id)
}

// ValidateJobID verifies a lowercase UUIDv4 job identifier.
func ValidateJobID(id string) bool {
	return jobIDRegex.MatchString(id)
}

// CanonicalVideoURL builds a safe YouTube watch URL from a validated video ID.
func CanonicalVideoURL(videoID string) (string, error) {
	if !ValidateVideoID(videoID) {
		return "", errors.New("invalid YouTube video identifier")
	}
	return "https://www.youtube.com/watch?v=" + videoID, nil
}

// SanitizeFilename strips unsafe path characters, control runes, and excessive length.
func SanitizeFilename(name string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 120
	}

	var b strings.Builder
	b.Grow(len(name))
	lastSpace := false

	for _, r := range strings.TrimSpace(name) {
		if unicode.IsControl(r) || r == 0 {
			continue
		}
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
		default:
			if unicode.IsSpace(r) {
				if !lastSpace {
					b.WriteRune(' ')
					lastSpace = true
				}
			} else {
				b.WriteRune(r)
				lastSpace = false
			}
		}
	}

	cleaned := strings.TrimSpace(b.String())
	cleaned = strings.Trim(cleaned, ".")
	for strings.Contains(cleaned, "..") {
		cleaned = strings.ReplaceAll(cleaned, "..", ".")
	}
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return "stash-media"
	}

	runes := []rune(cleaned)
	if len(runes) > maxRunes {
		cleaned = strings.TrimSpace(string(runes[:maxRunes]))
		cleaned = strings.TrimRight(cleaned, ".")
	}
	if cleaned == "" {
		return "stash-media"
	}
	return cleaned
}

// ContentDispositionAttachment produces an RFC 5987 safe attachment header value.
func ContentDispositionAttachment(filename string) string {
	ext := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)
	safeStem := SanitizeFilename(stem, 120)
	safeExt := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, ext)
	full := safeStem + safeExt

	// ASCII-only fallback for legacy clients
	var asciiFallback strings.Builder
	for _, r := range full {
		if r >= 0x20 && r <= 0x7E && r != '"' && r != '\\' && r != ';' {
			asciiFallback.WriteRune(r)
		} else {
			asciiFallback.WriteRune('_')
		}
	}
	fallback := strings.TrimSpace(asciiFallback.String())
	if fallback == "" {
		fallback = "download" + safeExt
	}

	encoded := url.PathEscape(full)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, fallback, encoded)
}

// EnsurePathWithinDir verifies targetPath resolves strictly beneath baseDir.
func EnsurePathWithinDir(baseDir, targetPath string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes workspace boundary")
	}
	return absTarget, nil
}

func splitCleanPath(p string) []string {
	var parts []string
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			parts = append(parts, seg)
		}
	}
	return parts
}
