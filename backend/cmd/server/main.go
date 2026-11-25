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

	if cfg.CookiesFile == "" && cfg.CookiesB64 != "" {
		cleanB64 := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, cfg.CookiesB64)
		decoded, err := base64.StdEncoding.DecodeString(cleanB64)
		if err != nil {
			logger.Error("cookies_b64_decode_failed", "error", err)
		} else {
			cookiePath := filepath.Join(os.TempDir(), "stash-cookies.txt")
			if err := os.WriteFile(cookiePath, decoded, 0o600); err != nil {
				logger.Error("cookies_write_failed", "error", err)
			} else {
				cfg.CookiesFile = cookiePath
			}
		}
	}

	rootCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	tooling := engine.DetectTooling(rootCtx, cfg)
	logger.Info("stash_backend_tooling",
		"yt_dlp_version", tooling.YtDlpVersion,
		"ffmpeg_available", tooling.FFmpegPath != "",
		"js_runtime", tooling.JSRuntimeName,
		"pot_provider_enabled", tooling.PotURL != "",
		"cookies_configured", tooling.CookiesFile != "",
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
