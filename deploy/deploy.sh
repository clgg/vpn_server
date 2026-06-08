#!/usr/bin/env bash
set -euo pipefail

APP_NAME="go-sqlite-api"
APP_DIR="/opt/apps/${APP_NAME}"
SERVICE_NAME="${APP_NAME}"
REMOTE_USER="${REMOTE_USER:-ubuntu}"
REMOTE_HOST="${REMOTE_HOST:?Set REMOTE_HOST to your server public IP or domain}"
SSH_KEY="${SSH_KEY:-${HOME}/.ssh/LightsailDefaultKey-ap-northeast-1.pem}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARCHIVE="/tmp/${APP_NAME}.tar.gz"

if command -v xattr >/dev/null 2>&1; then
  xattr -cr "${ROOT_DIR}" 2>/dev/null || true
fi

COPYFILE_DISABLE=1 tar --no-xattrs --no-mac-metadata --disable-copyfile \
  --exclude ".git" \
  --exclude "${APP_NAME}" \
  --exclude "*.db" \
  --exclude "*.db-shm" \
  --exclude "*.db-wal" \
  -czf "${ARCHIVE}" \
  -C "${ROOT_DIR}" .

scp -i "${SSH_KEY}" "${ARCHIVE}" "${REMOTE_USER}@${REMOTE_HOST}:/tmp/${APP_NAME}.tar.gz"

ssh -i "${SSH_KEY}" "${REMOTE_USER}@${REMOTE_HOST}" <<EOF
set -euo pipefail
sudo install -d -m 755 /opt/apps
sudo rm -rf "${APP_DIR}.new"
sudo install -d -m 755 "${APP_DIR}.new"
sudo tar -xzf "/tmp/${APP_NAME}.tar.gz" -C "${APP_DIR}.new"
sudo chown -R "${REMOTE_USER}:${REMOTE_USER}" "${APP_DIR}.new"
cd "${APP_DIR}.new"
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y libsqlite3-dev >/dev/null
CGO_ENABLED=1 go build -tags libsqlite3 -o "${APP_NAME}" .
sudo chown root:root "${APP_NAME}"
sudo chmod 755 "${APP_NAME}"
sudo useradd --system --home /var/lib/${APP_NAME} --shell /usr/sbin/nologin goapi 2>/dev/null || true
sudo install -d -o goapi -g goapi -m 750 /var/lib/${APP_NAME}
sudo rm -rf "${APP_DIR}.previous"
if [ -d "${APP_DIR}" ]; then sudo mv "${APP_DIR}" "${APP_DIR}.previous"; fi
sudo mv "${APP_DIR}.new" "${APP_DIR}"
sudo cp "${APP_DIR}/deploy/go-sqlite-api.service" /etc/systemd/system/go-sqlite-api.service
if [ ! -f /etc/go-sqlite-api/vpn.env ]; then
  sudo install -d -m 755 /etc/go-sqlite-api
  sudo cp "${APP_DIR}/deploy/vpn.env.example" /etc/go-sqlite-api/vpn.env
  sudo chmod 600 /etc/go-sqlite-api/vpn.env
  echo "WARNING: created /etc/go-sqlite-api/vpn.env from template; edit it before exporting client configs"
fi
sudo install -m 755 "${APP_DIR}/deploy/vpn-healthcheck.sh" /usr/local/sbin/vpn-healthcheck.sh
sudo cp "${APP_DIR}/deploy/vpn-healthcheck.service" /etc/systemd/system/vpn-healthcheck.service
sudo cp "${APP_DIR}/deploy/vpn-healthcheck.timer" /etc/systemd/system/vpn-healthcheck.timer
sudo cp "${APP_DIR}/deploy/nginx-go-sqlite-api.conf" /etc/nginx/sites-available/go-sqlite-api
sudo rm -f /etc/nginx/sites-enabled/default
sudo ln -sf /etc/nginx/sites-available/go-sqlite-api /etc/nginx/sites-enabled/go-sqlite-api
sudo nginx -t
sudo systemctl daemon-reload
sudo systemctl enable --now "${SERVICE_NAME}"
sudo systemctl enable --now vpn-healthcheck.timer
sudo systemctl restart "${SERVICE_NAME}"
sudo systemctl restart nginx
sudo systemctl is-active "${SERVICE_NAME}"
sudo systemctl is-active nginx
sudo systemctl is-active vpn-healthcheck.timer
EOF

ssh -i "${SSH_KEY}" "${REMOTE_USER}@${REMOTE_HOST}" \
  'curl -fsS http://127.0.0.1:8080/health'
echo
curl -fsS --max-time 10 "http://${REMOTE_HOST}/health" >/dev/null \
  || echo "public /health check did not complete; local service health is OK"
echo
