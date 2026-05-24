# Developing ProcessGit

This document covers the workflow for **modifying ProcessGit itself** — its Go source, templates, viewers, or release pipeline. If you only want to **deploy** ProcessGit, see the [Installation section in README.md](README.md#installation) instead.

## Repository structure

| Path | Contents |
|---|---|
| `cmd/`, `routers/`, `models/`, `modules/`, `services/` | Gitea fork — Go backend |
| `web_src/` | Frontend (Vue, Less, TypeScript) — assembled by webpack/esbuild |
| `templates/` | Server-rendered Go templates |
| `options/locale/` | Translation strings (per-language JSON) |
| `deploy/` | Production deployment: compose file, bootstrap scripts, Dockerfile |
| `updater/` | The self-update sidecar (separate Go module, stdlib-only) |
| `.github/workflows/` | CI and release automation |
| `docs/` | Feature documentation |

## Building locally

ProcessGit targets Go 1.25. Earlier Go toolchains can't parse `go.mod` directives like `godebug`, `tool`, or `ignore` blocks.

```bash
# Verify Go version
go version          # expect: go1.25.x

# Build the main binary
make build

# Run with the local config
./gitea web
```

For frontend changes:

```bash
make watch-frontend  # rebuilds on save
```

## Running tests

```bash
make test                 # Go unit tests
make test-frontend        # frontend tests
make lint                 # gofmt, golangci-lint, eslint, stylelint
make lint-backend-fix     # auto-fix what's auto-fixable
```

The updater sidecar lives in `updater/` and has its own test suite:

```bash
cd updater
go test ./...
```

## Building the deployment image locally

The `deploy/docker-compose.yml` is hardcoded to pull pre-built images from GHCR. To build and run from your local working tree instead:

```bash
docker compose -f deploy/docker-compose.yml up -d --build
```

This builds `deploy/Dockerfile.processgit` against the current source. Useful for iterating on Gitea-layer changes; not the supported production deployment path.

## Release process

Releases are tag-driven. Push a `vX.Y.Z` tag and the `.github/workflows/release.yml` workflow does the rest:

1. Builds `linux/amd64` Docker images for both `processgit` and `processgit-updater`
2. Pushes them to `ghcr.io/algomation-ai/{processgit,processgit-updater}` tagged with both `:X.Y.Z` and `:latest`
3. Signs each image with Sigstore cosign (keyless, OIDC-based)
4. Generates a `release.json` manifest containing image digests, then signs that too
5. Creates a GitHub Release with auto-generated notes (commits since previous tag) and the signed manifest as an asset

Workflow wall-clock is ~3–5 minutes with warm buildx cache, ~12 minutes from cold.

To tag a release:

```bash
git tag -a v0.1.5 -m "ProcessGit v0.1.5"
git push origin v0.1.5
```

The workflow runs on the tag push.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [CLA.md](CLA.md).

## Upstream Gitea relationship

ProcessGit is a soft fork of [Gitea](https://github.com/go-gitea/gitea). For the policy on tracking upstream changes, see [UPSTREAM.md](UPSTREAM.md).
