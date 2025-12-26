# syntax=docker/dockerfile:1

# Build stage with required Node + Go
FROM node:22.6.0-alpine3.22 AS build-env

ARG GOPROXY=direct
ARG GITEA_VERSION
ARG TAGS="sqlite sqlite_unlock_notify"
ENV TAGS="bindata timetzdata $TAGS"
ARG CGO_EXTRA_CFLAGS

# Build deps (Go + build tools)
RUN apk --no-cache add \
    go \
    build-base \
    git \
    bash \
    pnpm

WORKDIR /src

# Copy repo sources (exclude .git; we mount it for version info)
COPY --exclude=.git/ . .

# POC unblocker: some forks miss this file but webpack expects it
RUN mkdir -p assets && [ -f assets/go-licenses.json ] || echo "[]" > assets/go-licenses.json

# Build (requires BuildKit for --mount)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.local/share/pnpm/store \
    --mount=type=bind,source=.git,target=.git \
    make

# Bring in runtime rootfs (entrypoint + s6 services) from repo
COPY docker/root /tmp/local

# Ensure executable bits (helps if repo edited on Windows)
RUN chmod 755 /tmp/local/usr/bin/entrypoint \
              /tmp/local/usr/local/bin/* \
              /tmp/local/etc/s6/gitea/* \
              /tmp/local/etc/s6/openssh/* \
              /tmp/local/etc/s6/.s6-svscan/* \
              /src/gitea

# Runtime stage
FROM alpine:3.22 AS processgit

EXPOSE 22 3000

RUN apk --no-cache add \
    bash \
    ca-certificates \
    curl \
    gettext \
    git \
    linux-pam \
    openssh \
    s6 \
    sqlite \
    su-exec \
    gnupg

RUN addgroup -S -g 1000 git && \
    adduser  -S -H -D -h /data/git -s /bin/bash -u 1000 -G git git && \
    echo "git:*" | chpasswd -e

COPY --from=build-env /tmp/local /
COPY --from=build-env /src/gitea /app/gitea/gitea
COPY --from=build-env /src/templates /app/gitea/templates
COPY --from=build-env /src/public    /app/gitea/public

ENV USER=git
ENV GITEA_CUSTOM=/data/gitea

VOLUME ["/data"]

ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["/usr/bin/s6-svscan", "/etc/s6"]
