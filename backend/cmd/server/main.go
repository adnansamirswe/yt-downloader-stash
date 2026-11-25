package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	"stash-backend/internal/api"
	"stash-backend/internal/config"
	"stash-backend/internal/engine"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := config.Load()

	cookiesStatus := "not_set"
	hasFile := false
	if cfg.CookiesFile != "" {
		if st, err := os.Stat(cfg.CookiesFile); err == nil && !st.IsDir() && st.Size() > 0 {
			hasFile = true
			cookiesStatus = "file_ok"
		} else {
			cookiesStatus = "file_missing"
		}
	}

	if !hasFile && cfg.CookiesB64 != "" {
		decoded, err := decodeCookiesEnv(cfg.CookiesB64)
		if err != nil {
			cookiesStatus = "b64_decode_error"
			logger.Error("cookies_b64_decode_failed", "error", err)
		} else {
			cookiePath := filepath.Join(os.TempDir(), "stash-cookies-master.txt")
			_ = os.Remove(cookiePath)
			if err := os.WriteFile(cookiePath, decoded, 0o400); err != nil {
				cookiesStatus = "write_error"
				logger.Error("cookies_write_failed", "error", err)
			} else {
				cfg.CookiesFile = cookiePath
				cookiesStatus = "b64_ok"
			}
		}
	}

	rootCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	tooling := engine.DetectTooling(rootCtx, cfg)
	tooling.CookiesStatus = cookiesStatus
	logger.Info("stash_backend_tooling",
		"yt_dlp_version", tooling.YtDlpVersion,
		"ffmpeg_available", tooling.FFmpegPath != "",
		"js_runtime", tooling.JSRuntimeName,
		"pot_provider_enabled", tooling.PotURL != "",
		"cookies_configured", tooling.HasCookies(),
		"cookies_status", tooling.CookiesStatus,
	)

	inspector := engine.NewInspector(tooling, cfg.CacheTTL, logger)
	jobManager, err := engine.NewJobManager(rootCtx, cfg, tooling, logger)
	if err != nil {
		logger.Error("job_manager_init_failed", "error", err)
		os.Exit(1)
	}

	apiServer := api.NewServer(rootCtx, cfg, tooling, inspector, jobManager, logger)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout is intentionally 0 so long-running SSE telemetry and multi-GB ZIP streams never abort mid-transfer.
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("stash_backend_listening", "port", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		logger.Error("http_server_failed", "error", err)
		os.Exit(1)
	case <-rootCtx.Done():
		logger.Info("stash_backend_shutting_down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("http_shutdown_error", "error", err)
		}
	}
}

func decodeCookiesEnv(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	// Strip accidental surrounding quotes from .env copy-paste
	if len(trimmed) >= 2 && ((trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') || (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'')) {
		trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	}
	clean := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, trimmed)

	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, enc := range encodings {
		if b, err := enc.DecodeString(clean); err == nil && len(b) > 0 {
			return b, nil
		} else if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("empty cookies payload")
	}
	return nil, lastErr
}
