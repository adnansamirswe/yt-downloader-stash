package api

import (
	"net/http"
	"strings"

	"stash-backend/internal/model"
	"stash-backend/internal/security"
)

func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request) {
	if !s.allowRequest(r, s.inspectLimiter) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many inspect requests. Please wait a moment and try again.")
		return
	}

	var req model.InspectRequest
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "auto"
	}
	switch mode {
	case "auto", "video", "playlist":
	default:
		writeError(w, http.StatusBadRequest, "invalid_mode", "mode must be 'auto', 'video', or 'playlist'")
		return
	}

	validated, err := security.ValidateYouTubeURL(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_url", err.Error())
		return
	}

	res, err := s.inspector.Inspect(r.Context(), validated, mode)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "inspect_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}
