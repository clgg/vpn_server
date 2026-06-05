#!/usr/bin/env bash
set -euo pipefail

PORT="${VPN_SERVER_PORT:-443}"

if ! systemctl is-active --quiet sing-box; then
  systemctl restart sing-box
  exit 0
fi

if ! ss -lnt "sport = :${PORT}" | grep -q ":${PORT}"; then
  systemctl restart sing-box
  exit 0
fi

if ! curl -fsS --max-time 8 https://api.ipify.org >/dev/null; then
  logger -t vpn-healthcheck "outbound connectivity check failed; leaving sing-box running"
fi
