package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"stash-backend/internal/model"
)

const maxJSONBodyBytes = 1 << 20 // 1 MiB cap for API JSON payloads

type apiErrorBody struct {
	Error apiErrorDetail `json:"error"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiErrorBody{
		Error: apiErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// decodeStrictJSON decodes a single JSON object while rejecting unknown fields and oversized payloads.
func decodeStrictJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		return errors.New("Content-Type must be application/json")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return errors.New("request body exceeds maximum allowed size")
		}
		if errors.Is(err, io.EOF) {
			return errors.New("request body must not be empty")
		}
		return errors.New("invalid JSON request payload or unknown field supplied")
	}

	// Reject trailing data after the first JSON value
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func normalizeMediaParams(mediaType, quality, container, audioBitrate string) (string, string, string, string, error) {
	mt := strings.ToLower(strings.TrimSpace(mediaType))
	if mt != model.MediaTypeVideo && mt != model.MediaTypeAudio {
		return "", "", "", "", errors.New("mediaType must be either 'video' or 'audio'")
	}

	c := strings.ToLower(strings.TrimSpace(container))
	q := strings.ToLower(strings.TrimSpace(quality))
	br := strings.ToLower(strings.TrimSpace(audioBitrate))

	if mt == model.MediaTypeVideo {
		if c == "" {
			c = "mp4"
		}
		switch c {
		case "mp4", "mkv", "webm":
		default:
			return "", "", "", "", errors.New("video container must be mp4, mkv, or webm")
		}

		if q == "" {
			q = "1080p"
		}
		switch q {
		case "best", "2160p", "1440p", "1080p", "720p", "480p", "360p":
		default:
			return "", "", "", "", errors.New("video quality must be best, 2160p, 1440p, 1080p, 720p, 480p, or 360p")
		}
		return mt, q, c, "", nil
	}

	// Audio mode
	if c == "" {
		c = "m4a"
	}
	switch c {
	case "m4a", "opus", "mp3", "flac", "wav":
	default:
		return "", "", "", "", errors.New("audio container must be m4a, opus, mp3, flac, or wav")
	}

	if c == "mp3" {
		if br == "" {
			br = "320k"
		}
		switch br {
		case "320k", "192k", "128k":
		default:
			return "", "", "", "", errors.New("MP3 audioBitrate must be 320k, 192k, or 128k")
		}
	} else {
		br = ""
	}

	return mt, "", c, br, nil
}
