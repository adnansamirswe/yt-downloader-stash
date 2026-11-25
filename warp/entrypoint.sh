#!/bin/sh
set -eu

cd /config

if [ ! -f /config/wgcf-account.toml ]; then
  echo "[warp] Registering new Cloudflare WARP device..."
  wgcf register --accept-tos
fi

if [ ! -f /config/wgcf-profile.conf ]; then
  echo "[warp] Generating WireGuard profile..."
  wgcf generate
fi

# Strip IPv6 interface address and AllowedIPs if running in IPv4-only Docker bridge
if [ ! -f /config/wireproxy.conf ]; then
  cat > /config/wireproxy.conf <<'EOF'
WGConfig = /config/wgcf-profile.conf

[Socks5]
BindAddress = 0.0.0.0:1080
EOF
fi

echo "[warp] Starting userspace wireproxy SOCKS5 on :1080..."
exec wireproxy -c /config/wireproxy.conf
