#!/usr/bin/env bash
set -euo pipefail

PORT="${VPN_SERVER_PORT:-443}"

if ! systemctl is-active --quiet xray; then
  systemctl restart xray
  exit 0
fi

if ! ss -lnt "sport = :${PORT}" | grep -q ":${PORT}"; then
  systemctl restart xray
  exit 0
fi

if ! curl -fsS --max-time 8 https://api.ipify.org >/dev/null; then
  logger -t vpn-healthcheck "outbound connectivity check failed; leaving xray running"
fi
