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

# Build an IPv4-clean WireGuard config for wireproxy:
# - Pins Cloudflare's official IPv4 WARP endpoint (162.159.192.1:2408)
# - Keeps the IPv4 Address (172.16.0.2/32) and AllowedIPs (0.0.0.0/0) while stripping IPv6 entries
# - Adds PersistentKeepalive = 25 so VPS NAT never drops idle UDP sessions
awk '
  /^Endpoint[[:space:]]*=/ { print "Endpoint = 162.159.192.1:2408"; next }
  /^Address[[:space:]]*=/ {
    sub(/,.*/, "", $0)
    if ($0 !~ /:/) print $0
    next
  }
  /^AllowedIPs[[:space:]]*=/ {
    print "AllowedIPs = 0.0.0.0/0"
    next
  }
  /^PersistentKeepalive[[:space:]]*=/ { next }
  { print }
  END { print "PersistentKeepalive = 25" }
' /config/wgcf-profile.conf > /config/wgcf-ipv4.conf

cat > /config/wireproxy.conf <<'EOF'
WGConfig = /config/wgcf-ipv4.conf

[Socks5]
BindAddress = 0.0.0.0:1080

[http]
BindAddress = 0.0.0.0:8118
EOF

echo "[warp] Starting userspace wireproxy (IPv4 endpoint 162.159.192.1:2408; SOCKS5 :1080, HTTP :8118)..."
exec wireproxy -c /config/wireproxy.conf
