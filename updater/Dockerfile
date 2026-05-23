# syntax=docker/dockerfile:1.7
#
# processgit-updater — sidecar that orchestrates ProcessGit self-updates.
#
# The runtime image carries:
#   - the updater binary itself (stdlib-only Go, ~8 MB)
#   - the docker CLI (~80 MB) for talking to the bind-mounted /var/run/docker.sock
#   - cosign (~50 MB) for image and blob signature verification
#   - ca-certificates so we can hit api.github.com and ghcr.io
#
# Total uncompressed image: ~150 MB. The vast majority is the docker CLI; the
# updater itself is tiny.
#
# At runtime, /var/run/docker.sock must be bind-mounted from the host:
#   docker run -v /var/run/docker.sock:/var/run/docker.sock processgit-updater
#
# See deploy/docker-compose.yml (added in a follow-up PR) for the full
# compose-driven setup.

ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.22

# -----------------------------------------------------------------------------
# Stage 1: build the updater binary
# -----------------------------------------------------------------------------
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build
WORKDIR /src

# Cache module downloads separately so source changes don't bust them.
COPY go.mod ./
# (no go.sum because the updater is stdlib-only — see TestNoExternalImports)
RUN go mod download

# Now bring in source.
COPY *.go ./

# Build with version stamped from build arg.
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags "-s -w -X main.version=${VERSION}+${COMMIT}" \
      -o /out/processgit-updater \
      .

# -----------------------------------------------------------------------------
# Stage 2: cosign — copy from official Sigstore image
# -----------------------------------------------------------------------------
FROM gcr.io/projectsigstore/cosign:v2.4.1 AS cosign

# -----------------------------------------------------------------------------
# Stage 3: runtime
# -----------------------------------------------------------------------------
FROM alpine:${ALPINE_VERSION} AS runtime

RUN apk add --no-cache \
      ca-certificates \
      docker-cli \
      tini \
 && update-ca-certificates 2>/dev/null || true

# Cosign binary from the official Sigstore image.
COPY --from=cosign /ko-app/cosign /usr/local/bin/cosign

# The updater binary.
COPY --from=build /out/processgit-updater /usr/local/bin/processgit-updater

# Non-root user. We can't drop entirely because the docker CLI talks to the
# docker socket which is typically owned by root or `docker` group on the
# host. Compose users with rootless docker will need to adjust.
RUN addgroup -S updater && adduser -S -G updater updater

# State directory (persisted via a docker volume).
RUN mkdir -p /var/lib/processgit-updater \
 && chown updater:updater /var/lib/processgit-updater

USER updater
WORKDIR /var/lib/processgit-updater
EXPOSE 9000

# tini handles SIGTERM/SIGINT for graceful shutdown.
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/processgit-updater"]

LABEL org.opencontainers.image.title="processgit-updater"
LABEL org.opencontainers.image.description="ProcessGit self-update orchestration sidecar"
LABEL org.opencontainers.image.source="https://github.com/Algomation-AI/ProcessGit"
LABEL org.opencontainers.image.url="https://processgit.org"
LABEL org.opencontainers.image.vendor="Algomation-AI"
LABEL org.opencontainers.image.licenses="MIT"
