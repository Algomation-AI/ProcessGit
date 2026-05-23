// HTTP API. All endpoints except /healthz require a bearer token matching
// the value of $PROCESSGIT_UPDATER_TOKEN. The token is shared between the
// main app container (which calls the updater) and the updater itself, via
// the docker-compose .env file.
//
// The updater listens on 0.0.0.0 inside its container; access is gated
// purely by the docker-compose network, plus the bearer token as defence
// in depth in case someone exposes the port to the host.

package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type API struct {
	Token        string
	Store        *Store
	GitHub       *GitHubClient
	Orchestrator *Orchestrator
	Log          *slog.Logger
	Version      string // updater's own version
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.Handle("GET /status", a.auth(a.handleStatus))
	mux.Handle("GET /releases/latest", a.auth(a.handleLatestRelease))
	mux.Handle("POST /update", a.auth(a.handleUpdateStart))
	mux.Handle("GET /update/{id}", a.auth(a.handleUpdateGet))
	mux.Handle("GET /history", a.auth(a.handleHistory))
	return mux
}

// --- Middleware ---

func (a *API) auth(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(hdr, prefix) || subtle.ConstantTimeCompare([]byte(hdr[len(prefix):]), []byte(a.Token)) != 1 {
			httpError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}
		h(w, r)
	})
}

// --- Handlers ---

func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": a.Version,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"version":           a.Version,
		"active_job":        a.Store.Active(),
		"recent_jobs_count": len(a.Store.List()),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleLatestRelease(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	if channel == "" {
		channel = "stable"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	rel, err := a.GitHub.LatestRelease(ctx, channel)
	if err != nil {
		httpError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tag":          rel.TagName,
		"name":         rel.Name,
		"prerelease":   rel.Prerelease,
		"html_url":     rel.HTMLURL,
		"published_at": rel.PublishedAt,
	})
}

type updateStartRequest struct {
	TargetTag string `json:"target_tag"`
	// Future: Channel string, Strategy string
}

func (a *API) handleUpdateStart(w http.ResponseWriter, r *http.Request) {
	var req updateStartRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.TargetTag == "" {
		httpError(w, http.StatusBadRequest, "target_tag is required")
		return
	}
	job, err := a.Orchestrator.Start(r.Context(), req.TargetTag)
	if err != nil {
		if strings.Contains(err.Error(), "already in progress") {
			httpError(w, http.StatusConflict, err.Error())
			return
		}
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":     job.ID,
		"status_url": "/update/" + job.ID,
		"job":        job,
	})
}

func (a *API) handleUpdateGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, ok := a.Store.Get(id)
	if !ok {
		httpError(w, http.StatusNotFound, "no such job: "+id)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (a *API) handleHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"jobs": a.Store.List(),
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) {
		// nothing we can usefully do at this point
	}
}

func httpError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg, "code": code})
}
