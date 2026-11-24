package api

import (
	"net/http"

	"stash-backend/internal/model"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	potEnabled := s.tooling.PotURL != ""
	potHealthy := false
	if potEnabled {
		potHealthy = s.tooling.CheckPotProvider(r.Context())
	}

	status := "ok"
	if s.tooling.YtDlpPath == "" || s.tooling.FFmpegPath == "" {
		status = "degraded"
	}

	writeJSON(w, http.StatusOK, model.HealthResponse{
		Status:              status,
		YtDlpVersion:        s.tooling.YtDlpVersion,
		FFmpegAvailable:     s.tooling.FFmpegPath != "",
		FFmpegVersion:       s.tooling.FFmpegVersion,
		JSRuntime:           s.tooling.JSRuntimeName,
		PotProviderEnabled:  potEnabled,
		PotProviderHealthy:  potHealthy,
		MaxConcurrentTracks: s.cfg.MaxConcurrentTracks,
		ActiveJobs:          s.jobs.ActiveJobCount(),
	})
}
