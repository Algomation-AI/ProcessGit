// Docker CLI wrapper. Slice 3B replaces the Slice 3A stubs with real
// implementations:
//
//   Pull                      → docker pull <ref>
//   InspectAppImageDigest     → docker inspect → image ID → RepoDigests
//   RunMigration              → docker run --rm --volumes-from <app> <image> <cmd>
//   SwapContainer             → rewrite PROCESSGIT_VERSION in .env, then
//                               docker compose up -d --no-deps <app>
//   Healthcheck               → HTTP GET against the app health URL
//   Rollback                  → restore previous PROCESSGIT_VERSION + compose up
//   Snapshot                  → still deferred to Slice 3C; logs a warning
//
// Stub mode (`PROCESSGIT_UPDATER_STUB=true`) is preserved so existing tests
// keep working and operators can validate the architecture before flipping
// to real updates.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Docker struct {
	Bin          string
	AppContainer string
	ComposeFile  string // /deploy/docker-compose.yml inside the updater container
	EnvFile      string // /deploy/.env — for persisting PROCESSGIT_VERSION
	AppHealthURL string // e.g. http://processgit:3000/api/v1/version
	Stub         bool
	Log          *slog.Logger
}

// NewDocker keeps the 3-arg signature so existing tests don't break.
// The non-stub fields (ComposeFile, EnvFile, AppHealthURL) are set on the
// returned value directly by main.go.
func NewDocker(log *slog.Logger, appContainer string, stub bool) *Docker {
	return &Docker{
		Bin:          "docker",
		AppContainer: appContainer,
		Stub:         stub,
		Log:          log,
	}
}

// requireFields validates that the operation has everything it needs in
// non-stub mode. Called from each public method.
func (d *Docker) requireFields(forOp string, fields ...string) error {
	if d.Stub {
		return nil
	}
	for _, f := range fields {
		switch f {
		case "ComposeFile":
			if d.ComposeFile == "" {
				return fmt.Errorf("docker.%s: ComposeFile is required (set PROCESSGIT_UPDATER_COMPOSE_FILE)", forOp)
			}
		case "EnvFile":
			if d.EnvFile == "" {
				return fmt.Errorf("docker.%s: EnvFile is required (set PROCESSGIT_UPDATER_ENV_FILE)", forOp)
			}
		case "AppHealthURL":
			if d.AppHealthURL == "" {
				return fmt.Errorf("docker.%s: AppHealthURL is required (set PROCESSGIT_UPDATER_HEALTH_URL)", forOp)
			}
		}
	}
	return nil
}

// run executes a docker subcommand and returns combined stdout+stderr.
func (d *Docker) run(ctx context.Context, what string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, d.Bin, args...)
	cmd.Env = os.Environ()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("docker %s failed: %w; output: %s", what, err, strings.TrimSpace(buf.String()))
	}
	return buf.String(), nil
}

// --- Pull ------------------------------------------------------------------

func (d *Docker) Pull(ctx context.Context, ref string) error {
	d.Log.Info("docker.pull", "ref", ref, "stub", d.Stub)
	if d.Stub {
		return sleepCtx(ctx, 5*time.Second)
	}
	_, err := d.run(ctx, "pull "+ref, "pull", ref)
	return err
}

// --- InspectAppImageDigest -------------------------------------------------

// Returns the image digest currently in use by the app container. If the
// container is running an image built locally (and never pushed/pulled),
// there's no RepoDigest available — we return the image ID instead.
// Either way the value uniquely identifies the previous state and is
// suitable as a rollback reference.
func (d *Docker) InspectAppImageDigest(ctx context.Context) (string, error) {
	d.Log.Info("docker.inspect_app", "container", d.AppContainer, "stub", d.Stub)
	if d.Stub {
		return "sha256:0000000000000000000000000000000000000000000000000000000000000000", nil
	}
	out, err := d.run(ctx, "inspect container", "inspect", "--format", "{{.Image}}", d.AppContainer)
	if err != nil {
		return "", err
	}
	imageID := strings.TrimSpace(out)
	if imageID == "" {
		return "", fmt.Errorf("no image ID for container %q", d.AppContainer)
	}
	out, err = d.run(ctx, "inspect image", "inspect", "--format", "{{json .RepoDigests}}", imageID)
	if err != nil {
		return "", err
	}
	var digests []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &digests); err != nil {
		return "", fmt.Errorf("parse RepoDigests: %w", err)
	}
	if len(digests) == 0 {
		// Locally built image with no registry digest. Return image ID.
		return imageID, nil
	}
	// "registry/repo@sha256:abc..." — extract the part after '@'.
	first := digests[0]
	if at := strings.LastIndex(first, "@"); at > 0 && at < len(first)-1 {
		return first[at+1:], nil
	}
	return first, nil
}

// --- RunMigration ----------------------------------------------------------

// Runs the migration command inside a one-shot container from `image`,
// with --volumes-from <app_container> so it has access to the same data.
// The app container is NOT stopped here; migration commands are expected
// to be idempotent and tolerant of the app still serving (e.g. additive
// schema migrations). Migrations that require downtime should orchestrate
// a stop/run/start sequence in the manifest's migration.command itself.
func (d *Docker) RunMigration(ctx context.Context, image, command string) (string, error) {
	d.Log.Info("docker.run_migration", "image", image, "command", command, "stub", d.Stub)
	if d.Stub {
		if err := sleepCtx(ctx, 3*time.Second); err != nil {
			return "", err
		}
		return fmt.Sprintf("[stub] would have run: docker run --rm --volumes-from %s %s %s\n", d.AppContainer, image, command), nil
	}
	args := []string{
		"run", "--rm",
		"--volumes-from", d.AppContainer,
		image,
	}
	// command is a single string from the manifest (e.g. "/app/gitea/gitea migrate").
	// Split on whitespace — simple but adequate for the documented use case
	// (no quoted args in the manifest's migration.command field today).
	args = append(args, strings.Fields(command)...)
	return d.run(ctx, "run migration", args...)
}

// --- SwapContainer ---------------------------------------------------------

// Compose-driven swap. Updates PROCESSGIT_VERSION in the .env file, then
// runs `docker compose up -d --no-deps <app_container>`. Compose recreates
// only the app container, leaving sibling services (bootstrap, init-perms,
// updater itself) untouched.
//
// Returns the OLD container ID for diagnostic purposes — the actual rollback
// mechanism uses the previously-recorded PROCESSGIT_VERSION value rather
// than the container ID.
func (d *Docker) SwapContainer(ctx context.Context, newImage string) (string, error) {
	d.Log.Info("docker.swap", "new_image", newImage, "stub", d.Stub)
	if d.Stub {
		if err := sleepCtx(ctx, 4*time.Second); err != nil {
			return "", err
		}
		return "stub-old-container-id", nil
	}
	if err := d.requireFields("SwapContainer", "ComposeFile", "EnvFile"); err != nil {
		return "", err
	}

	// Capture the old container ID before we start the swap (logged in the Job).
	oldID, _ := d.run(ctx, "container inspect ID", "inspect", "--format", "{{.Id}}", d.AppContainer)
	oldID = strings.TrimSpace(oldID)

	newTag := imageVersion(newImage)
	if newTag == "" {
		return oldID, fmt.Errorf("could not derive version tag from image %q", newImage)
	}

	if _, _, err := SetEnvFileKey(d.EnvFile, "PROCESSGIT_VERSION", newTag); err != nil {
		return oldID, fmt.Errorf("update %s: %w", d.EnvFile, err)
	}

	out, err := d.run(ctx, "compose up --no-deps",
		"compose", "-f", d.ComposeFile,
		"up", "-d", "--no-deps", d.AppContainer,
	)
	if err != nil {
		d.Log.Error("compose up failed", "output", out)
		return oldID, err
	}
	d.Log.Info("compose up complete", "new_tag", newTag, "output_tail", lastLines(out, 10))
	return oldID, nil
}

// --- Healthcheck -----------------------------------------------------------

// Polls the app's health endpoint until 200 OK or a 2-minute deadline.
// The endpoint comes from $PROCESSGIT_UPDATER_HEALTH_URL — typically
// `http://processgit:3000/api/v1/version` from inside the compose network.
func (d *Docker) Healthcheck(ctx context.Context) error {
	d.Log.Info("docker.healthcheck", "url", d.AppHealthURL, "stub", d.Stub)
	if d.Stub {
		return sleepCtx(ctx, 3*time.Second)
	}
	if err := d.requireFields("Healthcheck", "AppHealthURL"); err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(2 * time.Minute)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.AppHealthURL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		if err := sleepCtx(ctx, 3*time.Second); err != nil {
			return err
		}
	}
	return fmt.Errorf("healthcheck timed out after 2m; last error: %v", lastErr)
}

// --- Rollback --------------------------------------------------------------

// Restores the previously-running image by writing the previous
// PROCESSGIT_VERSION back to .env and running compose up again.
func (d *Docker) Rollback(ctx context.Context, oldContainerID, oldImage string) error {
	d.Log.Info("docker.rollback", "old_container", oldContainerID, "old_image", oldImage, "stub", d.Stub)
	if d.Stub {
		return sleepCtx(ctx, 2*time.Second)
	}
	if err := d.requireFields("Rollback", "ComposeFile", "EnvFile"); err != nil {
		return err
	}

	rollbackTag := imageVersion(oldImage)
	if rollbackTag == "" {
		// oldImage looks like a digest (no tag part). We don't have a
		// reliable way to determine the previous PROCESSGIT_VERSION from a
		// raw digest, so we fall back to "latest" — best-effort recovery.
		d.Log.Warn("rollback target has no tag form; falling back to latest", "old_image", oldImage)
		rollbackTag = "latest"
	}
	if _, _, err := SetEnvFileKey(d.EnvFile, "PROCESSGIT_VERSION", rollbackTag); err != nil {
		return fmt.Errorf("revert %s: %w", d.EnvFile, err)
	}
	_, err := d.run(ctx, "compose up rollback",
		"compose", "-f", d.ComposeFile,
		"up", "-d", "--no-deps", d.AppContainer,
	)
	return err
}

// --- Snapshot (still deferred to Slice 3C) --------------------------------

func (d *Docker) Snapshot(ctx context.Context, dst string) error {
	d.Log.Info("docker.snapshot", "dst", dst, "stub", d.Stub)
	if d.Stub {
		return sleepCtx(ctx, 2*time.Second)
	}
	// Slice 3C will implement:
	//   docker run --rm --volumes-from <app> -v <dst_dir>:/snap alpine \
	//     tar czf /snap/data-<jobid>.tgz /data
	// For now we log and succeed without taking a real snapshot. The
	// rollback path is still correct (it just reverts to the previous
	// image tag), it just doesn't restore data if the migration mutated it.
	d.Log.Warn("snapshot not implemented in Slice 3B; continuing without on-disk backup",
		"dst", dst)
	return nil
}

// --- Helpers ---------------------------------------------------------------

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// imageVersion extracts the tag portion from a docker image ref.
//
//	"ghcr.io/foo/bar:0.1.2"          → "0.1.2"
//	"ghcr.io/foo/bar@sha256:abc..."  → "" (digest form has no tag)
//	"0.1.2"                          → "0.1.2"
//	"registry:5000/foo:0.1"          → "0.1"  (handles port-in-registry)
func imageVersion(ref string) string {
	if strings.Contains(ref, "@") {
		return ""
	}
	// Take the substring after the last "/", so we don't confuse a port in
	// the registry hostname with the tag separator.
	after := ref
	if slash := strings.LastIndex(ref, "/"); slash >= 0 {
		after = ref[slash+1:]
	}
	if colon := strings.LastIndex(after, ":"); colon >= 0 {
		return after[colon+1:]
	}
	return ref
}

// lastLines returns the last n lines of s, joined by '\n'. Used to truncate
// long docker-compose output in log entries.
func lastLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	parts := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(parts) <= n {
		return strings.Join(parts, "\n")
	}
	return strings.Join(parts[len(parts)-n:], "\n")
}
