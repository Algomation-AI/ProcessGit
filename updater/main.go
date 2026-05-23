// processgit-updater
//
// The updater sidecar. Exposes an HTTP API for the main ProcessGit
// container to ask "are there updates?" and "please apply this update".
// Orchestrates the actual update via docker.sock.
//
// Env config:
//
//   PROCESSGIT_UPDATER_TOKEN          (required) bearer token for the HTTP API
//   PROCESSGIT_UPDATER_LISTEN         default ":9000"
//   PROCESSGIT_UPDATER_STATE_DIR      default "/var/lib/processgit-updater"
//   PROCESSGIT_UPDATER_REPO           default "Algomation-AI/ProcessGit"
//   PROCESSGIT_UPDATER_GITHUB_API     default "https://api.github.com"
//   PROCESSGIT_UPDATER_GITHUB_TOKEN   optional; raises rate limit, allows private repos
//   PROCESSGIT_UPDATER_APP_CONTAINER  default "processgit"
//   PROCESSGIT_UPDATER_STUB           "true" to run with docker ops stubbed (Slice 3A default)
//
// Version is set at build time via -ldflags "-X main.version=..."

package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	listen := envOr("PROCESSGIT_UPDATER_LISTEN", ":9000")
	stateDir := envOr("PROCESSGIT_UPDATER_STATE_DIR", "/var/lib/processgit-updater")
	repo := envOr("PROCESSGIT_UPDATER_REPO", "Algomation-AI/ProcessGit")
	githubAPI := envOr("PROCESSGIT_UPDATER_GITHUB_API", "https://api.github.com")
	githubToken := os.Getenv("PROCESSGIT_UPDATER_GITHUB_TOKEN")
	appContainer := envOr("PROCESSGIT_UPDATER_APP_CONTAINER", "processgit")
	stub := envBool("PROCESSGIT_UPDATER_STUB", true) // Slice 3A: stub by default
	token := os.Getenv("PROCESSGIT_UPDATER_TOKEN")

	// Allow -version for sanity
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		writeStdout("processgit-updater " + version + "\n")
		os.Exit(0)
	}

	logLevel := slog.LevelInfo
	if envBool("PROCESSGIT_UPDATER_DEBUG", false) {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(log)

	if token == "" {
		log.Error("PROCESSGIT_UPDATER_TOKEN is required; refusing to start")
		os.Exit(2)
	}
	if !strings.Contains(repo, "/") {
		log.Error("PROCESSGIT_UPDATER_REPO must be OWNER/NAME", "value", repo)
		os.Exit(2)
	}

	statePath := filepath.Join(stateDir, "state.json")
	store, err := NewStore(statePath)
	if err != nil {
		log.Error("init store", "err", err, "path", statePath)
		os.Exit(1)
	}

	gh := NewGitHubClient(githubAPI, repo, githubToken)
	cosign := NewCosign()
	docker := NewDocker(log.With("component", "docker"), appContainer, stub)
	orch := &Orchestrator{
		Store:    store,
		GitHub:   gh,
		Cosign:   cosign,
		Docker:   docker,
		Log:      log.With("component", "orchestrator"),
		StateDir: stateDir,
	}

	api := &API{
		Token:        token,
		Store:        store,
		GitHub:       gh,
		Orchestrator: orch,
		Log:          log.With("component", "api"),
		Version:      version,
	}

	srv := &http.Server{
		Addr:              listen,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	log.Info("processgit-updater starting",
		"version", version,
		"listen", listen,
		"state_dir", stateDir,
		"repo", repo,
		"app_container", appContainer,
		"stub_mode", stub,
	)

	// Listen & shut down gracefully on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server crashed", "err", err)
			os.Exit(1)
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	log.Info("clean shutdown")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func writeStdout(s string) {
	_, _ = os.Stdout.WriteString(s)
}
