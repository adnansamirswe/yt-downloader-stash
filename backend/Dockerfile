# syntax=docker/dockerfile:1

FROM docker.io/denoland/deno:bin AS deno

FROM docker.io/library/golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -p 2 -trimpath -ldflags="-s -w" -o /out/stash-api ./cmd/server

FROM docker.io/library/debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    ffmpeg \
    python3 \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Install latest yt-dlp with default extras + bgutil PO-token plugin into an isolated venv
RUN python3 -m venv /opt/yt-dlp-venv \
    && /opt/yt-dlp-venv/bin/pip install --no-cache-dir -U pip \
    && /opt/yt-dlp-venv/bin/pip install --no-cache-dir -U "yt-dlp[default]" bgutil-ytdlp-pot-provider \
    && ln -s /opt/yt-dlp-venv/bin/yt-dlp /usr/local/bin/yt-dlp

# Bundle official Deno binary for yt-dlp 2026 JS challenge solving
COPY --from=deno /deno /usr/local/bin/deno

# Copy static Go API binary
COPY --from=builder /out/stash-api /usr/local/bin/stash-api

RUN groupadd --gid 10001 stash \
    && useradd --uid 10001 --gid stash --shell /usr/sbin/nologin --create-home stash \
    && mkdir -p /tmp/stash-jobs /data \
    && chown -R stash:stash /tmp/stash-jobs /data

USER stash
WORKDIR /home/stash

ENV STASH_PORT=4000 \
    STASH_TEMP_DIR=/tmp/stash-jobs \
    STASH_JS_RUNTIME=deno:/usr/local/bin/deno \
    STASH_TRUST_PROXY=true \
    GOMEMLIMIT=256MiB

EXPOSE 4000

HEALTHCHECK --interval=30s --timeout=5s --start-period=25s --retries=5 \
    CMD curl -fsS http://127.0.0.1:4000/api/v1/health || exit 1

ENTRYPOINT ["/usr/local/bin/stash-api"]
