# Nevatal Cloud backend image: three binaries, one image.
#
#   /app/nevatal-api      control-plane API (default command, port 8090)
#   /app/nevatal-migrate  one-off database migration runner
#   /app/nevatal-edge     subdomain edge proxy for https://<name>.<PUBLIC_DOMAIN_ROOT>
#                         (port EDGE_PORT, default 8088; run with
#                         command: ["/app/nevatal-edge"], see docker-compose.yml)
#
# Multi-stage build: compiles inside the image, so a clean clone builds with
# nothing but Docker (BuildKit, the default builder since Docker 23).
#
#   docker build -t nevatal-cloud-backend --build-arg VERSION=1.2.0 backend/
#
# Cross-build for the amd64 Proxmox host from an arm64 workstation:
#
#   docker buildx build --platform linux/amd64 -t <registry>/nevatal-cloud-backend:<tag> --push backend/
#
# SQL migrations are embedded into both binaries with go:embed
# (backend/migrations/migrations.go), so the runtime image does not need the
# migrations directory.

ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.24

# ---------------------------------------------------------------- builder ---
# The builder always runs on the build host's native platform; Go
# cross-compiles to TARGETOS/TARGETARCH (no QEMU emulation needed).
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

# Dependency layer: only re-downloaded when go.mod/go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH
# Empty = keep the version compiled into internal/config (config.Version).
ARG VERSION=

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eux; \
    export CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}"; \
    ldflags="-s -w"; \
    if [ -n "${VERSION}" ]; then ldflags="${ldflags} -X github.com/nevatal/cloud/internal/config.Version=${VERSION}"; fi; \
    go build -trimpath -ldflags "${ldflags}" -o /out/nevatal-api ./cmd/api; \
    go build -trimpath -ldflags "${ldflags}" -o /out/nevatal-migrate ./cmd/migrate; \
    go build -trimpath -ldflags "${ldflags}" -o /out/nevatal-edge ./cmd/edge

# ---------------------------------------------------------------- runtime ---
FROM alpine:${ALPINE_VERSION}

ARG VERSION=
LABEL org.opencontainers.image.title="nevatal-cloud-backend" \
      org.opencontainers.image.description="Nevatal Cloud control-plane API for Proxmox VE" \
      org.opencontainers.image.source="https://github.com/thirza258/nevatal-cloud" \
      org.opencontainers.image.version="${VERSION}"

# ca-certificates: TLS to Proxmox (PROXMOX_VERIFY_TLS=true) and OpenRouter.
# tzdata: correct local time in logs/schedules. wget: container healthcheck.
RUN set -eux; \
    apk add --no-cache ca-certificates tzdata wget; \
    addgroup -S -g 10001 nevatal; \
    adduser -S -D -H -u 10001 -G nevatal -h /app -s /sbin/nologin nevatal; \
    mkdir -p /data/storage; \
    chown -R 10001:10001 /data; \
    chmod 0750 /data/storage

WORKDIR /app
COPY --from=builder /out/nevatal-api /out/nevatal-migrate /out/nevatal-edge /app/

ENV PORT=8090 \
    STORAGE_PATH=/data/storage

# Object-storage files live here: mount a persistent volume (shared RWX/NFS
# when running more than one API replica).
USER 10001:10001

# 8090 = API, 8088 = edge (only when the container runs /app/nevatal-edge).
EXPOSE 8090 8088

# Works for both long-running binaries: the API answers /health on PORT, the
# edge answers /__edge/health on EDGE_PORT. Compose and Kubernetes define
# their own per-service checks; this one serves a plain `docker run`.
HEALTHCHECK --interval=15s --timeout=5s --start-period=60s --retries=4 \
    CMD wget -qO /dev/null "http://127.0.0.1:${PORT:-8090}/health" 2>/dev/null \
        || wget -qO /dev/null "http://127.0.0.1:${EDGE_PORT:-8088}/__edge/health" \
        || exit 1

# The API and the edge drain in-flight requests on SIGTERM; give the API
# >= 60s (compose: stop_grace_period, Kubernetes: terminationGracePeriodSeconds).
STOPSIGNAL SIGTERM
CMD ["/app/nevatal-api"]
