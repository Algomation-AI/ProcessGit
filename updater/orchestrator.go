// The orchestrator runs the update state machine. It owns the active Job
// throughout its lifecycle and is the single goroutine that mutates the Job
// fields between persistence calls.
//
// One update at a time, period. Concurrent updates would race for
// /var/run/docker.sock, the database migration, and the container swap.
// The store's AddJob enforces this.

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"
)

type Orchestrator struct {
	Store    *Store
	GitHub   *GitHubClient
	Cosign   *Cosign
	Docker   *Docker
	Log      *slog.Logger
	StateDir string
}

// Start kicks off an update for the given target tag. Returns the new Job
// immediately (the actual work runs in a goroutine). The caller polls
// /update/{id} for status.
func (o *Orchestrator) Start(ctx context.Context, targetTag string) (*Job, error) {
	if targetTag == "" {
		return nil, errors.New("target_tag is required")
	}
	job := NewJob(targetTag)
	if err := o.Store.AddJob(job); err != nil {
		return nil, err
	}

	// Run the update on its own context derived from a background root, so
	// the request's cancellation doesn't kill an update mid-flight. The
	// orchestrator has its own deadline.
	go o.run(context.WithoutCancel(ctx), job)
	return job, nil
}

// run drives the job through its state machine. Each phase is wrapped so a
// returned error fails the job (and triggers rollback for phases that come
// after the container swap).
func (o *Orchestrator) run(ctx context.Context, job *Job) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	log := o.Log.With("job_id", job.ID, "target", job.TargetTag)
	log.Info("update.start")
	defer func() {
		_ = o.Store.ClearActive()
		log.Info("update.end", "final_state", job.State, "error", job.Error)
	}()

	// === planning ===
	step := job.transitionTo(StatePlanning)
	manifest, err := o.planUpdate(ctx, job, log)
	if err != nil {
		step.Finish("", err)
		job.fail(StateRolledBack, fmt.Errorf("planning failed: %w", err))
		_ = o.Store.PersistActive()
		return
	}
	step.Finish(fmt.Sprintf("target %s digest %s", manifest.Image.Ref(), manifest.Image.Digest), nil)
	job.TargetVersion = manifest.Version
	job.TargetImage = manifest.Image.Ref()
	job.TargetDigest = manifest.Image.Digest
	_ = o.Store.PersistActive()

	// === snapshotting ===
	step = job.transitionTo(StateSnapshotting)
	snapPath := filepath.Join(o.StateDir, "snapshots", job.ID+".tar")
	if err := o.Docker.Snapshot(ctx, snapPath); err != nil {
		step.Finish("", err)
		job.fail(StateRolledBack, fmt.Errorf("snapshot failed: %w", err))
		_ = o.Store.PersistActive()
		return
	}
	step.Finish("snapshot at "+snapPath, nil)
	_ = o.Store.PersistActive()

	// === pulling ===
	step = job.transitionTo(StatePulling)
	if err := o.Docker.Pull(ctx, manifest.Image.DigestRef()); err != nil {
		step.Finish("", err)
		job.fail(StateRolledBack, fmt.Errorf("image pull failed: %w", err))
		_ = o.Store.PersistActive()
		return
	}
	step.Finish("pulled "+manifest.Image.DigestRef(), nil)
	_ = o.Store.PersistActive()

	// === verifying (after pull; cosign needs the image in the local registry to verify) ===
	step = job.transitionTo(StateVerifying)
	if err := o.Cosign.VerifyImage(ctx, manifest); err != nil {
		step.Finish("", err)
		job.fail(StateRolledBack, fmt.Errorf("cosign image verify failed: %w", err))
		_ = o.Store.PersistActive()
		return
	}
	// (the blob verify already happened in planUpdate; this is the image verify)
	step.Finish("cosign verify passed", nil)
	_ = o.Store.PersistActive()

	// === migrating ===
	if manifest.Migration.Required {
		step = job.transitionTo(StateMigrating)
		out, err := o.Docker.RunMigration(ctx, manifest.Image.DigestRef(), manifest.Migration.Command)
		if err != nil {
			step.Finish(out, err)
			job.fail(StateRolledBack, fmt.Errorf("migration failed: %w", err))
			_ = o.Store.PersistActive()
			return
		}
		step.Finish(out, nil)
		_ = o.Store.PersistActive()
	}

	// Capture the currently-running image so we can roll back if healthcheck fails.
	prevDigest, err := o.Docker.InspectAppImageDigest(ctx)
	if err != nil {
		log.Warn("could not capture rollback target", "err", err)
	}
	job.PreviousImage = prevDigest

	// === swapping ===
	step = job.transitionTo(StateSwapping)
	oldContainerID, err := o.Docker.SwapContainer(ctx, manifest.Image.DigestRef())
	if err != nil {
		step.Finish("", err)
		job.fail(StateRolledBack, fmt.Errorf("container swap failed: %w", err))
		_ = o.Store.PersistActive()
		return
	}
	step.Finish("swapped (old container id "+oldContainerID+")", nil)
	_ = o.Store.PersistActive()

	// === healthchecking — beyond this point, failure means we must roll back ===
	step = job.transitionTo(StateHealthchecking)
	if err := o.Docker.Healthcheck(ctx); err != nil {
		step.Finish("", err)
		o.rollback(ctx, job, oldContainerID, prevDigest, log)
		return
	}
	step.Finish("healthy", nil)
	_ = o.Store.PersistActive()

	// === committed ===
	job.transitionTo(StateCommitted)
	job.Steps[len(job.Steps)-1].Finish("committed", nil)
	_ = o.Store.PersistActive()
}

func (o *Orchestrator) planUpdate(ctx context.Context, job *Job, log *slog.Logger) (*Manifest, error) {
	rel, err := o.GitHub.ReleaseByTag(ctx, job.TargetTag)
	if err != nil {
		return nil, fmt.Errorf("fetch release %s: %w", job.TargetTag, err)
	}
	fm, err := o.GitHub.FetchManifest(ctx, rel)
	if err != nil {
		return nil, err
	}
	// Verify the manifest signature BEFORE trusting any of its contents.
	if err := o.Cosign.VerifyBlob(ctx, fm.Manifest, fm.JSONBytes, fm.Sig, fm.Cert); err != nil {
		return nil, fmt.Errorf("release.json signature verification failed: %w", err)
	}
	log.Info("planned",
		"target_tag", fm.Manifest.Tag,
		"image_digest", fm.Manifest.Image.Digest,
		"migration_required", fm.Manifest.Migration.Required,
	)
	return fm.Manifest, nil
}

// rollback is the failure path entered when healthcheck fails AFTER the
// container swap. If rollback itself fails we end up in StateFailed, which
// is the only state that requires human intervention.
func (o *Orchestrator) rollback(ctx context.Context, job *Job, oldContainerID, oldImage string, log *slog.Logger) {
	step := job.transitionTo(StateRollingBack)
	log.Warn("healthcheck failed — rolling back",
		"old_container", oldContainerID, "old_image", oldImage)
	if err := o.Docker.Rollback(ctx, oldContainerID, oldImage); err != nil {
		step.Finish("", err)
		job.fail(StateFailed, fmt.Errorf("rollback failed (manual intervention needed): %w", err))
		_ = o.Store.PersistActive()
		return
	}
	step.Finish("rolled back to "+oldImage, nil)
	job.transitionTo(StateRolledBack)
	job.Steps[len(job.Steps)-1].Finish("rolled back successfully; update aborted", nil)
	job.Error = "healthcheck failed after swap; rolled back to previous image"
	_ = o.Store.PersistActive()
}
