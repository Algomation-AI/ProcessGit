package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Store -----------------------------------------------------------------

func TestStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}

	j := NewJob("v1.2.3")
	if err := s.AddJob(j); err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	if got := s.Active(); got == nil || got.ID != j.ID {
		t.Fatalf("Active should be the just-added job; got %+v", got)
	}

	// Adding a second job while one is active must fail.
	if err := s.AddJob(NewJob("v1.2.4")); err == nil {
		t.Fatal("expected AddJob to refuse while another is active")
	}

	// Mark terminal & persist, then a new job should be accepted.
	j.transitionTo(StateCommitted)
	j.Steps[len(j.Steps)-1].Finish("done", nil)
	if err := s.PersistActive(); err != nil {
		t.Fatal(err)
	}
	if got := s.Active(); got != nil {
		t.Fatalf("Active should be nil after terminal; got %+v", got)
	}

	j2 := NewJob("v1.2.4")
	if err := s.AddJob(j2); err != nil {
		t.Fatalf("AddJob after terminal: %v", err)
	}

	// Reload from disk and confirm both jobs are present, ordering preserved.
	s2, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	list := s2.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(list))
	}
	if list[0].TargetTag != "v1.2.4" || list[1].TargetTag != "v1.2.3" {
		t.Fatalf("wrong ordering after reload: %s, %s", list[0].TargetTag, list[1].TargetTag)
	}
	if got := s2.Active(); got == nil || got.ID != j2.ID {
		t.Fatalf("active not restored after reload; got %+v", got)
	}
}

func TestState_IsTerminal(t *testing.T) {
	terminal := map[State]bool{
		StateCommitted: true, StateRolledBack: true, StateFailed: true, StateAborted: true,
		StateIdle: false, StatePlanning: false, StateSnapshotting: false,
		StatePulling: false, StateVerifying: false, StateMigrating: false,
		StateSwapping: false, StateHealthchecking: false, StateRollingBack: false,
	}
	for s, want := range terminal {
		if got := s.IsTerminal(); got != want {
			t.Errorf("%s.IsTerminal() = %v, want %v", s, got, want)
		}
	}
}

func TestJob_TransitionFinishesOnTerminal(t *testing.T) {
	j := NewJob("v1.0.0")
	j.transitionTo(StatePlanning)
	if j.CompletedAt != nil {
		t.Fatal("CompletedAt should be nil on non-terminal")
	}
	j.transitionTo(StateCommitted)
	if j.CompletedAt == nil {
		t.Fatal("CompletedAt should be set on terminal")
	}
}

// --- Orchestrator (stub docker) --------------------------------------------

// withDocker overrides the orchestrator's docker so we can simulate failure
// paths.
type fakeDocker struct {
	*Docker
	failPull     error
	failHealth   error
	failRollback error
	migrationOut string
}

func newOrch(t *testing.T) (*Orchestrator, *Store, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	docker := NewDocker(log, "processgit", true)

	gh := newFakeGitHub(t)
	gh.Manifest = sampleManifest()

	cosign := newFakeCosign()

	orch := &Orchestrator{
		Store:    store,
		GitHub:   NewGitHubClient(gh.Server.URL, "Algomation-AI/ProcessGit", ""),
		Cosign:   cosign.Cosign,
		Docker:   docker,
		Log:      log,
		StateDir: dir,
	}
	return orch, store, gh.Server
}

// fakeGitHub serves the minimum endpoints the orchestrator uses, with
// a configurable manifest and asset bodies.
type fakeGitHub struct {
	Server   *httptest.Server
	Manifest *Manifest
	// jsonBytes is the byte representation served as release.json (lets us
	// inject malformed payloads in tests).
	JSONBytes []byte
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	fg := &fakeGitHub{}
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/Algomation-AI/ProcessGit/releases/tags/", func(w http.ResponseWriter, r *http.Request) {
		// fabricate a release pointing to our fixture asset URLs (served below)
		base := "http://" + r.Host
		rel := ghRelease{
			TagName:    "v0.9.9",
			Name:       "v0.9.9",
			Draft:      false,
			Prerelease: false,
			HTMLURL:    "https://example.com/release",
			Assets: []ghAsset{
				{Name: "release.json", BrowserDownloadURL: base + "/dl/release.json"},
				{Name: "release.json.sig", BrowserDownloadURL: base + "/dl/release.json.sig"},
				{Name: "release.json.crt", BrowserDownloadURL: base + "/dl/release.json.crt"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/dl/release.json", func(w http.ResponseWriter, r *http.Request) {
		if fg.JSONBytes != nil {
			_, _ = w.Write(fg.JSONBytes)
			return
		}
		b, _ := json.Marshal(fg.Manifest)
		_, _ = w.Write(b)
	})
	mux.HandleFunc("/dl/release.json.sig", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-sig"))
	})
	mux.HandleFunc("/dl/release.json.crt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake-crt"))
	})
	fg.Server = httptest.NewServer(mux)
	t.Cleanup(fg.Server.Close)
	return fg
}

func sampleManifest() *Manifest {
	return &Manifest{
		SchemaVersion: 1,
		Name:          "processgit",
		Version:       "0.9.9",
		Tag:           "v0.9.9",
		ReleasedAt:    "2026-05-23T18:00:00Z",
		Prerelease:    false,
		Image: Image{
			Registry:   "ghcr.io",
			Repository: "algomation-ai/processgit",
			Tag:        "0.9.9",
			Digest:     "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			Platforms:  []string{"linux/amd64"},
		},
		Signing: Signing{
			Method:        "cosign-keyless",
			Issuer:        "https://token.actions.githubusercontent.com",
			IdentityRegex: "^https://github.com/Algomation-AI/.*",
		},
		ReleaseNotesURL: "https://example.com/release",
		Migration:       Migration{Required: false},
		BreakingChanges: []string{},
		Deprecations:    []string{},
	}
}

// fakeCosign replaces the cosign binary with a script that always succeeds.
// We use exec.Command's PATH lookup against a temp dir containing a stub
// executable named "cosign" that exits 0.
type fakeCosignSetup struct {
	Cosign *Cosign
}

func newFakeCosign() *fakeCosignSetup {
	// We can't trivially stub exec.Command, but we can point Cosign.Bin at
	// /bin/true which exists on all Linux test runners and accepts any args.
	return &fakeCosignSetup{Cosign: &Cosign{Bin: "/bin/true"}}
}

func TestOrchestrator_HappyPath(t *testing.T) {
	orch, store, _ := newOrch(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	job, err := orch.Start(ctx, "v0.9.9")
	if err != nil {
		t.Fatal(err)
	}
	if job.State != StateIdle {
		t.Fatalf("initial state should be idle, got %s", job.State)
	}

	// Wait for terminal.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := store.Get(job.ID)
		if got != nil && got.State.IsTerminal() {
			if got.State != StateCommitted {
				t.Fatalf("expected committed, got %s; error=%s", got.State, got.Error)
			}
			if got.TargetVersion != "0.9.9" {
				t.Errorf("TargetVersion not captured: %q", got.TargetVersion)
			}
			if got.TargetImage == "" || got.TargetDigest == "" {
				t.Errorf("TargetImage/Digest not captured: %+v", got)
			}
			// Make sure we passed through expected states.
			seen := map[State]bool{}
			for _, s := range got.Steps {
				seen[s.State] = true
			}
			for _, s := range []State{StatePlanning, StateSnapshotting, StatePulling, StateVerifying, StateSwapping, StateHealthchecking, StateCommitted} {
				if !seen[s] {
					t.Errorf("step %s missing from history", s)
				}
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("orchestrator never reached terminal state")
}

func TestOrchestrator_RejectsConcurrent(t *testing.T) {
	orch, _, _ := newOrch(t)
	ctx := context.Background()
	if _, err := orch.Start(ctx, "v0.9.9"); err != nil {
		t.Fatal(err)
	}
	_, err := orch.Start(ctx, "v0.9.8")
	if err == nil {
		t.Fatal("expected second concurrent Start to fail")
	}
}

// --- API -------------------------------------------------------------------

func TestAPI_BearerAuth(t *testing.T) {
	orch, store, _ := newOrch(t)
	api := &API{
		Token:        "supersecret",
		Store:        store,
		GitHub:       orch.GitHub,
		Orchestrator: orch,
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version:      "test",
	}
	srv := httptest.NewServer(api.Routes())
	defer srv.Close()

	// healthz: no auth required
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("healthz: status=%v err=%v", resp.StatusCode, err)
	}

	// /status without auth: 401
	resp, err = http.Get(srv.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	// /status with bad token: 401
	req, _ := http.NewRequest("GET", srv.URL+"/status", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 with wrong token, got %d", resp.StatusCode)
	}

	// /status with right token: 200
	req, _ = http.NewRequest("GET", srv.URL+"/status", nil)
	req.Header.Set("Authorization", "Bearer supersecret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}
}

// --- Sanity: ensure we didn't import anything outside stdlib -------------

func TestNoExternalImports(t *testing.T) {
	// This is enforced structurally by go.mod; the test is here as a
	// belt-and-suspenders marker. If go.sum exists, fail loudly.
	if _, err := os.Stat("go.sum"); err == nil {
		t.Fatal("go.sum exists — updater has acquired an external dependency; review carefully")
	}
}
