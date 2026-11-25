# syntax=docker/dockerfile:1

FROM docker.io/library/alpine:3.21

ARG TARGETARCH
ARG WGCF_VERSION=2.3.1
ARG WIREPROXY_VERSION=1.1.3

RUN apk add --no-cache ca-certificates curl tar \
    && ARCH="${TARGETARCH:-amd64}" \
    && curl -fsSL "https://github.com/ViRb3/wgcf/releases/download/v${WGCF_VERSION}/wgcf_${WGCF_VERSION}_linux_${ARCH}" -o /usr/local/bin/wgcf \
    && chmod +x /usr/local/bin/wgcf \
    && curl -fsSL "https://github.com/windtf/wireproxy/releases/download/v${WIREPROXY_VERSION}/wireproxy_linux_${ARCH}.tar.gz" \
       | tar -xz -C /usr/local/bin wireproxy \
    && chmod +x /usr/local/bin/wireproxy \
    && adduser -D -u 10002 warp \
    && mkdir -p /config \
    && chown -R warp:warp /config

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER warp
WORKDIR /config

EXPOSE 1080 8118

HEALTHCHECK --interval=30s --timeout=10s --start-period=20s --retries=5 \
    CMD curl -fsS -x socks5h://127.0.0.1:1080 --max-time 8 https://www.cloudflare.com/cdn-cgi/trace | grep -q "warp=on" || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
