package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"stash-backend/internal/config"
	"stash-backend/internal/engine"
)

func newTestAPIHandler(t *testing.T) http.Handler {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := config.Load()
	cfg.TempDir = t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tooling := &engine.Tooling{
		YtDlpPath:     "/bin/true",
		YtDlpVersion:  "2026.11.23",
		FFmpegPath:    "/bin/true",
		FFmpegVersion: "ffmpeg 6.1",
		JSRuntimeName: "node",
	}
	inspector := engine.NewInspector(tooling, cfg.CacheTTL, logger)
	jobs, err := engine.NewJobManager(ctx, cfg, tooling, logger)
	if err != nil {
		t.Fatal(err)
	}
	return NewServer(ctx, cfg, tooling, inspector, jobs, logger).Handler()
}

func TestHealthEndpoint(t *testing.T) {
	h := newTestAPIHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing security header X-Content-Type-Options")
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", payload["status"])
	}
}

func TestInspectRejectsUnknownFieldsAndSSRF(t *testing.T) {
	h := newTestAPIHandler(t)

	// Unknown field rejection
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/inspect", bytes.NewBufferString(`{"url":"https://youtu.be/dQw4w9WgXcQ","sneaky":true}`))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown field, got %d (%s)", rec1.Code, rec1.Body.String())
	}

	// SSRF domain rejection
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/inspect", bytes.NewBufferString(`{"url":"http://169.254.169.254/latest/meta-data/"}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for SSRF URL, got %d (%s)", rec2.Code, rec2.Body.String())
	}
}

func TestPlaylistJobRejectsEmptyAndBadVideoID(t *testing.T) {
	h := newTestAPIHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/playlist", bytes.NewBufferString(`{
		"mediaType": "audio",
		"container": "m4a",
		"items": [{"videoId": "../../bad", "title": "Oops", "index": 1}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid videoId, got %d (%s)", rec.Code, rec.Body.String())
	}
}
