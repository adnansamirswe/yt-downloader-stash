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

# Always write wireproxy.conf so both SOCKS5 (:1080) and HTTP (:8118) listeners stay up to date
cat > /config/wireproxy.conf <<'EOF'
WGConfig = /config/wgcf-profile.conf

[Socks5]
BindAddress = 0.0.0.0:1080

[http]
BindAddress = 0.0.0.0:8118
EOF

echo "[warp] Starting userspace wireproxy (SOCKS5 :1080, HTTP :8118)..."
exec wireproxy -c /config/wireproxy.conf
