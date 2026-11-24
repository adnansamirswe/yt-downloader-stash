package api

import (
	"context"
	"log/slog"
	"net/http"

	"stash-backend/internal/config"
	"stash-backend/internal/engine"
	"stash-backend/internal/security"
)

// Server wires HTTP routes, middleware, validation, inspection, and job orchestration.
type Server struct {
	cfg            config.Config
	tooling        *engine.Tooling
	inspector      *engine.Inspector
	jobs           *engine.JobManager
	logger         *slog.Logger
	inspectLimiter *security.IPRateLimiter
	jobLimiter     *security.IPRateLimiter
}

// NewServer constructs an API server ready to serve /api/v1/* endpoints.
func NewServer(ctx context.Context, cfg config.Config, tooling *engine.Tooling, inspector *engine.Inspector, jobs *engine.JobManager, logger *slog.Logger) *Server {
	return &Server{
		cfg:            cfg,
		tooling:        tooling,
		inspector:      inspector,
		jobs:           jobs,
		logger:         logger,
		inspectLimiter: security.NewIPRateLimiter(ctx, cfg.InspectRatePerMinute, 15),
		jobLimiter:     security.NewIPRateLimiter(ctx, cfg.JobRatePerMinute, 8),
	}
}

// Handler builds the full HTTP handler chain with security headers, CORS, and structured logging.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/inspect", s.handleInspect)
	mux.HandleFunc("POST /api/v1/jobs/single", s.handleStartSingleJob)
	mux.HandleFunc("POST /api/v1/jobs/playlist", s.handleStartPlaylistJob)
	mux.HandleFunc("GET /api/v1/jobs/{jobId}", s.handleGetJob)
	mux.HandleFunc("GET /api/v1/jobs/{jobId}/events", s.handleJobEvents)
	mux.HandleFunc("POST /api/v1/jobs/{jobId}/cancel", s.handleCancelJob)
	mux.HandleFunc("POST /api/v1/jobs/{jobId}/tracks/{videoId}/retry", s.handleRetryTrack)
	mux.HandleFunc("GET /api/v1/jobs/{jobId}/download", s.handleDownloadJob)
	mux.HandleFunc("GET /api/v1/jobs/{jobId}/tracks/{videoId}/download", s.handleDownloadTrack)

	// Catch-all JSON 404 for unknown API paths
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Endpoint not found.")
	})

	var h http.Handler = mux
	h = security.CORS(s.cfg.AllowedOrigins)(h)
	h = security.SecurityHeaders(h)
	h = security.RequestLogger(s.logger)(h)
	return h
}

func (s *Server) allowRequest(r *http.Request, limiter *security.IPRateLimiter) bool {
	if limiter == nil {
		return true
	}
	ip := security.ClientIP(r, s.cfg.TrustProxy)
	return limiter.Allow(ip)
}
