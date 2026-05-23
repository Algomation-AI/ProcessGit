# processgit-updater

Sidecar that lets a ProcessGit Docker deployment update itself: pull the
new image, verify its cosign signature, run the migration, swap the
running container, healthcheck, and roll back on failure.

A separate process from the main ProcessGit container because:

1. **A container can't safely update itself in place.** Replacing the
   running binary while it serves requests races. The sidecar runs
   continuously and survives across main-container restarts.
2. **Privilege boundary.** The updater needs `/var/run/docker.sock`. The
   main ProcessGit container should not.
3. **Tiny dependency surface.** Stdlib-only Go + docker CLI + cosign.
   Reviewable on its own.

## Architecture at a glance

```
              ┌──────────────────────┐         ┌──────────────────────┐
HTTP from     │      processgit      │  HTTP   │  processgit-updater  │
admin UI ───▶ │   (main app)         │ ──────▶ │   (this sidecar)     │
              │                      │ bearer  │                      │
              │  routers/web/admin/  │         │   /healthz           │
              │    updates.go        │         │   /status            │
              │  (Slice 4)           │         │   /releases/latest   │
              └──────────────────────┘         │   POST /update       │
                                                │   GET /update/{id}   │
                                                │   /history           │
                                                └─────────┬────────────┘
                                                          │ docker.sock
                                                          ▼
                                          ┌───────────────────────────────┐
                                          │   docker daemon on the host    │
                                          │   (pull / run / inspect / …)   │
                                          └───────────────────────────────┘
```

## Update state machine

```
idle → planning → snapshotting → pulling → verifying → migrating → swapping
                                                                     ↓
                                                              healthchecking
                                                                ↓             ↘ fail
                                                          committed       rolling_back
                                                          (success)            ↓
                                                                          rolled_back       (recovered)
                                                                              ↓ rollback fails
                                                                            failed          (manual intervention)
```

One update at a time, enforced by `Store.AddJob`. Concurrent attempts return
HTTP 409 Conflict.

**Critical safety property:** the manifest signature is verified BEFORE we
trust any of its fields (image ref, digest, migration command). An attacker
who can substitute a malicious `release.json` cannot redirect the updater
to a different image, because the cosign verification of the manifest blob
must pass first, and that's bound to the workflow's OIDC identity.

## Configuration

All via environment variables.

| Env var | Default | Purpose |
|---|---|---|
| `PROCESSGIT_UPDATER_TOKEN` | (required) | Bearer token for the HTTP API. Generate once with `openssl rand -hex 32`. Must match the value the main app uses to call the updater. |
| `PROCESSGIT_UPDATER_LISTEN` | `:9000` | Address to bind. |
| `PROCESSGIT_UPDATER_STATE_DIR` | `/var/lib/processgit-updater` | Where `state.json` lives. Bind-mount a volume. |
| `PROCESSGIT_UPDATER_REPO` | `Algomation-AI/ProcessGit` | GitHub repo to query for releases. |
| `PROCESSGIT_UPDATER_GITHUB_API` | `https://api.github.com` | Override for GitHub Enterprise. |
| `PROCESSGIT_UPDATER_GITHUB_TOKEN` | `""` | Optional. Raises rate limit; required for private repos. |
| `PROCESSGIT_UPDATER_APP_CONTAINER` | `processgit` | Name of the main app container, used for the swap. |
| `PROCESSGIT_UPDATER_STUB` | `true` | **Slice 3A default.** Skips real docker calls and simulates each phase with a short sleep. Set to `false` once Slice 3B lands. |
| `PROCESSGIT_UPDATER_DEBUG` | `false` | Verbose structured logs. |

## HTTP API

All paths except `/healthz` require `Authorization: Bearer $TOKEN`.

| Method + path | Body | Response |
|---|---|---|
| `GET /healthz` | — | `{status, version, time}` |
| `GET /status` | — | `{version, active_job, recent_jobs_count}` |
| `GET /releases/latest?channel=stable` | — | `{tag, name, prerelease, html_url, published_at}` |
| `POST /update` | `{target_tag: "v0.1.2"}` | 202 `{job_id, status_url, job}` (409 if another job is active) |
| `GET /update/{id}` | — | Job object (state, steps, errors) |
| `GET /history` | — | `{jobs: [...]}` (last 50, newest first) |

## Running locally for development

```bash
# Stub mode (default) — runs the state machine end-to-end without docker
PROCESSGIT_UPDATER_TOKEN=devtoken \
PROCESSGIT_UPDATER_STATE_DIR=/tmp/pg-updater \
PROCESSGIT_UPDATER_STUB=true \
go run .

# In another terminal
TOKEN=devtoken
curl -s http://localhost:9000/healthz | jq

curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:9000/releases/latest | jq

curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"target_tag": "v0.1.0"}' \
  http://localhost:9000/update | jq

# Note the job_id, then poll:
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:9000/update/$JOB_ID | jq '.state, .steps[-1]'
```

In stub mode the full state-machine run takes ~20 seconds.

In non-stub mode the planning + signature-verification phases still require
network access to `api.github.com` and to the OCI registry holding the image
being verified. cosign also reaches out to Sigstore's Rekor transparency log.

## Tests

```bash
go test -v ./...
```

Covers store round-trip, terminal-state classification, the orchestrator
happy path (full state-machine traversal in stub mode), concurrent-update
rejection, bearer auth on the API, and an external-dependency tripwire.

## Scope of this PR (Slice 3A)

What ships:

- Full HTTP API with bearer-token auth
- Update orchestrator with all states wired
- Real GitHub release fetching (`api.github.com/repos/…/releases`)
- Real cosign image + release.json blob verification (`cosign verify`,
  `cosign verify-blob` via `os/exec`)
- Atomic write-then-rename for the state file
- 7 tests covering store, state machine, API auth, no-deps invariant

What's stubbed (planned for Slice 3B):

- `docker pull`
- `docker run` for migrations
- The container swap (`docker stop` + `docker run` with carried-over
  config) — non-trivial, deserves its own focused PR
- `docker exec` health probing
- Rollback container restoration

Stub mode is the default precisely so the architecture can be reviewed and
the HTTP API exercised before any real container surgery is wired up.

## Scope of follow-ups

| Slice | Item |
|---|---|
| 3B | Real `docker pull` / `run` / `swap` / `healthcheck` / `rollback` |
| 3C | Volume snapshot + restore for full disaster recovery |
| 3D | Migration runner integration (handles the `Migration.Command` from the manifest robustly) |
| 4 | Admin UI page at `/-/admin/updates` that consumes this API |
| Workflow | Add `processgit-updater` image build to `.github/workflows/release.yml` so it ships paired with the main image |
| Deploy | `deploy/docker-compose.yml` integration — add the updater service, wire the bearer token via `.env`, set the network so only the main app can reach it |
