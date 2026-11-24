# AGENTS.md — Stash Engineering Guidelines

## Design Source of Truth

All UI, visual identity, typography, color tokens, motion, layout, and accessibility rules live in [`DESIGN.md`](./DESIGN.md). Read and follow `DESIGN.md` before modifying any frontend UI.

## Strict Architectural Separation

- **`backend/`** — Standalone **Go** HTTP API and media orchestration engine (`net/http`, `yt-dlp`, `ffmpeg`, `archive/zip` streaming, SSE telemetry, rate limiting, security validation). Owns all external YouTube requests, media processing, temp files, and API endpoints under `/api/v1/*`.
- **`frontend/`** — Standalone **Next.js 16** pure UI/UX client. Contains **no** `yt-dlp`, **no** `ffmpeg`, **no** server-side media download handlers, and **no** Next.js API routes duplicating backend actions. Communicates exclusively with `backend/` via `/api/v1/*` (same-origin behind `gateway` in Docker/Dokploy, or `NEXT_PUBLIC_API_URL` in local dev).
- **`gateway/`** — Minimal unbuffered Caddy reverse proxy uniting `frontend/` (`/*`) and `backend/` (`/api/*` with `flush_interval -1`) into a single Dokploy / Docker Compose entrypoint.

## Local Development Commands

- **Backend (Go)**:
  ```bash
  cd backend && go run ./cmd/server
  # Verify:
  cd backend && go vet ./... && go test -race -v ./...
  ```
- **Frontend (Next.js)**:
  ```bash
  cd frontend && NEXT_PUBLIC_API_URL=http://localhost:4000 npm run dev
  # Verify:
  cd frontend && npx tsc --noEmit && npm run lint && npm run build
  ```
- **Full Stack Container Build & Run (Local)**:
  ```bash
  docker compose -f docker-compose.yml -f docker-compose.local.yml up --build
  ```
- **Dokploy Deployment**:
  Deploy `docker-compose.yml` as a single Docker Compose service in Dokploy and map your domain in the Domains tab to service **`gateway`** on port **`8080`**.

## Security Rules

- Validate all inputs strictly at the Go backend boundary (`DisallowUnknownFields`, YouTube host allowlist against SSRF, strict `videoId` regex, `MaxBytesReader`).
- Never build shell commands by string concatenation; always pass argument arrays with `--` before URLs.
- Never commit `.env` files, cookies files, or secrets; keep only `.env.example` templates in Git.
- Kill entire subprocess groups (`Setpgid: true`) on job cancellation and clean up temporary job directories automatically.
