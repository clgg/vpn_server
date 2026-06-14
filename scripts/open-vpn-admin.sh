#!/usr/bin/env bash
set -euo pipefail

SSH_KEY="${SSH_KEY:-${HOME}/.ssh/LightsailDefaultKey-ap-northeast-1.pem}"
REMOTE_HOST="${REMOTE_HOST:-54.150.9.209}"
LOCAL_PORT="${LOCAL_PORT:-8787}"
URL="http://127.0.0.1:${LOCAL_PORT}/vpn-admin"

if [[ ! -f "${SSH_KEY}" ]]; then
  echo "SSH key not found: ${SSH_KEY}" >&2
  exit 1
fi

if lsof -nP -iTCP:"${LOCAL_PORT}" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "Tunnel already running: ${URL}"
  open "${URL}" 2>/dev/null || true
  exit 0
fi

echo "Starting SSH tunnel (do not close this terminal)..."
ssh -f -N \
  -i "${SSH_KEY}" \
  -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 \
  -L "${LOCAL_PORT}:127.0.0.1:80" \
  "ubuntu@${REMOTE_HOST}"

sleep 1
echo "Open: ${URL}"
open "${URL}" 2>/dev/null || true
echo
echo "To stop the tunnel later:"
echo "  lsof -ti tcp:${LOCAL_PORT} | xargs kill"
