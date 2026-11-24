# syntax=docker/dockerfile:1

FROM docker.io/library/caddy:2-alpine
COPY Caddyfile /etc/caddy/Caddyfile
EXPOSE 8080

HEALTHCHECK --interval=20s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1 || exit 1
