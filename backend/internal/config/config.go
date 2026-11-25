package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration for the Stash backend service.
type Config struct {
	Port                 int
	AllowedOrigins       []string
	TempDir              string
	MaxConcurrentJobs    int
	MaxConcurrentTracks  int
	FragmentsSingle      int
	FragmentsPlaylist    int
	MaxPlaylistItems     int
	CacheTTL             time.Duration
	JobTTL               time.Duration
	TrustProxy           bool
	YtDlpBin             string
	FFmpegBin            string
	JSRuntime            string
	PotProviderURL       string
	CookiesFile          string
	CookiesB64           string
	ProxyURL             string
	InspectRatePerMinute float64
	JobRatePerMinute     float64
}

// Load reads configuration from environment variables with safe defaults.
func Load() Config {
	defaultTemp := filepath.Join(os.TempDir(), "stash-jobs")

	originsRaw := getEnv("STASH_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")
	var origins []string
	for _, part := range strings.Split(originsRaw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return Config{
		Port:                 getEnvInt("STASH_PORT", 4000, 1, 65535),
		AllowedOrigins:       origins,
		TempDir:              getEnv("STASH_TEMP_DIR", defaultTemp),
		MaxConcurrentJobs:    getEnvInt("STASH_MAX_CONCURRENT_JOBS", 4, 1, 16),
		MaxConcurrentTracks:  getEnvInt("STASH_MAX_CONCURRENT_TRACKS", 4, 1, 6),
		FragmentsSingle:      getEnvInt("STASH_FRAGMENTS_SINGLE", 8, 1, 16),
		FragmentsPlaylist:    getEnvInt("STASH_FRAGMENTS_PLAYLIST", 4, 1, 8),
		MaxPlaylistItems:     getEnvInt("STASH_MAX_PLAYLIST_ITEMS", 200, 1, 500),
		CacheTTL:             time.Duration(getEnvInt("STASH_CACHE_TTL_SECONDS", 600, 30, 3600)) * time.Second,
		JobTTL:               time.Duration(getEnvInt("STASH_JOB_TTL_MINUTES", 30, 5, 240)) * time.Minute,
		TrustProxy:           getEnvBool("STASH_TRUST_PROXY", true),
		YtDlpBin:             strings.TrimSpace(os.Getenv("STASH_YTDLP_BIN")),
		FFmpegBin:            strings.TrimSpace(os.Getenv("STASH_FFMPEG_BIN")),
		JSRuntime:            strings.TrimSpace(os.Getenv("STASH_JS_RUNTIME")),
		PotProviderURL:       strings.TrimSpace(os.Getenv("STASH_POT_PROVIDER_URL")),
		CookiesFile:          strings.TrimSpace(os.Getenv("STASH_COOKIES_FILE")),
		CookiesB64:           strings.TrimSpace(os.Getenv("STASH_COOKIES_B64")),
		ProxyURL:             strings.TrimSpace(os.Getenv("STASH_PROXY_URL")),
		InspectRatePerMinute: float64(getEnvInt("STASH_INSPECT_RPM", 60, 5, 600)),
		JobRatePerMinute:     float64(getEnvInt("STASH_JOB_RPM", 30, 2, 300)),
	}
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback, minVal, maxVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if n < minVal {
		return minVal
	}
	if n > maxVal {
		return maxVal
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
