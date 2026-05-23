// Docker CLI wrapper. Slice 3A ships these as stubs that log + sleep — the
// state machine, HTTP API, and cosign verification can all be exercised
// end-to-end without touching real containers, which is what we want for
// the first iteration.
//
// Slice 3B will replace each method body with `exec.CommandContext(ctx,
// "docker", ...)` invocations that talk to /var/run/docker.sock through
// the bind-mounted docker CLI.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Docker struct {
	Bin            string // path to docker CLI; default "docker"
	AppContainer   string // e.g. "processgit"
	StagingNetwork string // optional separate network for the staged container
	Stub           bool   // when true, all operations are simulated
	Log            *slog.Logger
}

func NewDocker(log *slog.Logger, appContainer string, stub bool) *Docker {
	return &Docker{
		Bin:          "docker",
		AppContainer: appContainer,
		Stub:         stub,
		Log:          log,
	}
}

// Pull pulls the image at ref. In stub mode it logs and sleeps proportional
// to the size of an average ProcessGit image (~5s).
func (d *Docker) Pull(ctx context.Context, ref string) error {
	d.Log.Info("docker.pull", "ref", ref, "stub", d.Stub)
	if d.Stub {
		return sleepCtx(ctx, 5*time.Second)
	}
	return fmt.Errorf("docker.Pull: not implemented yet (Slice 3B)")
}

// Inspect returns the image digest currently in use by the app container,
// for capturing the rollback target.
func (d *Docker) InspectAppImageDigest(ctx context.Context) (string, error) {
	d.Log.Info("docker.inspect_app", "container", d.AppContainer, "stub", d.Stub)
	if d.Stub {
		// Return a deterministic stub digest so the state file shows something useful.
		return "sha256:0000000000000000000000000000000000000000000000000000000000000000", nil
	}
	return "", fmt.Errorf("docker.Inspect: not implemented yet (Slice 3B)")
}

// RunMigration runs `docker run --rm <image> <migration command>` and returns
// its combined output.
func (d *Docker) RunMigration(ctx context.Context, image, command string) (string, error) {
	d.Log.Info("docker.run_migration", "image", image, "command", command, "stub", d.Stub)
	if d.Stub {
		if err := sleepCtx(ctx, 3*time.Second); err != nil {
			return "", err
		}
		return fmt.Sprintf("[stub] would have run: docker run --rm %s %s\n", image, command), nil
	}
	return "", fmt.Errorf("docker.RunMigration: not implemented yet (Slice 3B)")
}

// SwapContainer stops the running app container and starts a new one from
// `newImage`, preserving env/volumes/network. Returns the old container ID
// so the orchestrator can roll back if healthcheck fails.
func (d *Docker) SwapContainer(ctx context.Context, newImage string) (oldContainerID string, err error) {
	d.Log.Info("docker.swap", "new_image", newImage, "stub", d.Stub)
	if d.Stub {
		if err := sleepCtx(ctx, 4*time.Second); err != nil {
			return "", err
		}
		return "stub-old-container-id", nil
	}
	return "", fmt.Errorf("docker.SwapContainer: not implemented yet (Slice 3B)")
}

// Healthcheck polls the new app container's /api/healthz endpoint until it
// returns 200 OK or the context expires.
func (d *Docker) Healthcheck(ctx context.Context) error {
	d.Log.Info("docker.healthcheck", "stub", d.Stub)
	if d.Stub {
		return sleepCtx(ctx, 3*time.Second)
	}
	return fmt.Errorf("docker.Healthcheck: not implemented yet (Slice 3B)")
}

// Rollback restores the previously running container.
func (d *Docker) Rollback(ctx context.Context, oldContainerID, oldImage string) error {
	d.Log.Info("docker.rollback", "old_container", oldContainerID, "old_image", oldImage, "stub", d.Stub)
	if d.Stub {
		return sleepCtx(ctx, 2*time.Second)
	}
	return fmt.Errorf("docker.Rollback: not implemented yet (Slice 3B)")
}

// Snapshot captures the state of the app's persistent volumes to a tarball
// in the snapshot directory. Slice 3C will use `docker run` against a
// tar-toolbox image; for Slice 3A we just log.
func (d *Docker) Snapshot(ctx context.Context, dst string) error {
	d.Log.Info("docker.snapshot", "dst", dst, "stub", d.Stub)
	if d.Stub {
		return sleepCtx(ctx, 2*time.Second)
	}
	return fmt.Errorf("docker.Snapshot: not implemented yet (Slice 3C)")
}

// sleepCtx sleeps for d, returning early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
