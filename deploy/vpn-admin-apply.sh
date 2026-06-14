#!/usr/bin/env bash
set -euo pipefail

XRAY_CANDIDATE="${VPN_XRAY_CANDIDATE:-/var/lib/go-sqlite-api/xray-config.json}"
HY2_CANDIDATE="${VPN_HY2_CANDIDATE:-/var/lib/go-sqlite-api/hysteria2-config.yaml}"
XRAY_CONFIG="/usr/local/etc/xray/config.json"
HY2_CONFIG="/etc/hysteria/config.yaml"
HY2_TLS_DIR="/etc/hysteria"
HY2_TLS_CERT="${VPN_HY2_TLS_CERT:-${HY2_TLS_DIR}/server.crt}"
HY2_TLS_KEY="${VPN_HY2_TLS_KEY:-${HY2_TLS_DIR}/server.key}"

if [ ! -f "${XRAY_CANDIDATE}" ]; then
  echo "xray candidate config does not exist: ${XRAY_CANDIDATE}" >&2
  exit 2
fi
if [ ! -f "${HY2_CANDIDATE}" ]; then
  echo "hysteria2 candidate config does not exist: ${HY2_CANDIDATE}" >&2
  exit 2
fi

/usr/local/bin/xray run -test -config "${XRAY_CANDIDATE}"

install -d -m 755 "${HY2_TLS_DIR}"
if [ ! -f "${HY2_TLS_CERT}" ] || [ ! -f "${HY2_TLS_KEY}" ]; then
  openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
    -keyout "${HY2_TLS_KEY}" \
    -out "${HY2_TLS_CERT}" \
    -subj "/CN=$(hostname -f 2>/dev/null || echo vpn-server)"
  chmod 600 "${HY2_TLS_KEY}"
  chmod 644 "${HY2_TLS_CERT}"
fi

install -o root -g root -m 644 "${XRAY_CANDIDATE}" "${XRAY_CONFIG}"
install -o root -g root -m 644 "${HY2_CANDIDATE}" "${HY2_CONFIG}"

# Legacy WireGuard setup redirected UDP 443/53 to 51820 and broke Hysteria2.
while iptables -t nat -C PREROUTING -i ens5 -p udp --dport 443 -j REDIRECT --to-ports 51820 2>/dev/null; do
  iptables -t nat -D PREROUTING -i ens5 -p udp --dport 443 -j REDIRECT --to-ports 51820
done
while iptables -t nat -C PREROUTING -i ens5 -p udp --dport 53 -j REDIRECT --to-ports 51820 2>/dev/null; do
  iptables -t nat -D PREROUTING -i ens5 -p udp --dport 53 -j REDIRECT --to-ports 51820
done
if [ -f /etc/wireguard/wg0.conf ]; then
  python3 - <<'PY' || true
from pathlib import Path
p = Path("/etc/wireguard/wg0.conf")
text = p.read_text()
for rule in (
    "iptables -t nat -C PREROUTING -i ens5 -p udp --dport 443 -j REDIRECT --to-ports 51820 2>/dev/null || iptables -t nat -A PREROUTING -i ens5 -p udp --dport 443 -j REDIRECT --to-ports 51820",
    "iptables -t nat -C PREROUTING -i ens5 -p udp --dport 53 -j REDIRECT --to-ports 51820 2>/dev/null || iptables -t nat -A PREROUTING -i ens5 -p udp --dport 53 -j REDIRECT --to-ports 51820",
    "iptables -t nat -D PREROUTING -i ens5 -p udp --dport 443 -j REDIRECT --to-ports 51820 2>/dev/null || true",
    "iptables -t nat -D PREROUTING -i ens5 -p udp --dport 53 -j REDIRECT --to-ports 51820 2>/dev/null || true",
):
    text = text.replace(rule, "")
text = text.replace(";;", ";")
text = text.replace("; ;", "; ")
p.write_text(text)
PY
fi

systemctl stop sing-box 2>/dev/null || true
systemctl disable sing-box 2>/dev/null || true

systemctl enable xray
systemctl restart xray
systemctl is-active --quiet xray

if systemctl list-unit-files 'hysteria-server.service' --no-legend 2>/dev/null | grep -q hysteria-server; then
  systemctl enable hysteria-server
  systemctl restart hysteria-server
  systemctl is-active --quiet hysteria-server
else
  echo "hysteria-server.service not found; install hysteria2 first" >&2
  exit 3
fi

HY2_DIR="$(dirname "${HY2_CONFIG}")"
for extra in "${HY2_DIR}"/config-*.yaml; do
  [ -f "${extra}" ] || continue
  base="$(basename "${extra}")"
  port="${base#config-}"
  port="${port%.yaml}"
  case "${port}" in
    ''|*[!0-9]*)
      continue
      ;;
  esac
  unit="hysteria-server-${port}.service"
  if [ ! -f "/etc/systemd/system/${unit}" ]; then
    sed "s|/etc/hysteria/config.yaml|${extra}|g; s|Hysteria2 Server|Hysteria2 Server (UDP ${port})|g" \
      /etc/systemd/system/hysteria-server.service >"/etc/systemd/system/${unit}"
  fi
  systemctl daemon-reload
  systemctl enable "${unit}"
  systemctl restart "${unit}"
  systemctl is-active --quiet "${unit}"
done

for candidate in /var/lib/go-sqlite-api/hysteria2-config-*.yaml; do
  [ -f "${candidate}" ] || continue
  base="$(basename "${candidate}")"
  port="${base#hysteria2-config-}"
  port="${port%.yaml}"
  case "${port}" in
    ''|*[!0-9]*)
      continue
      ;;
  esac
  install -o root -g root -m 644 "${candidate}" "${HY2_DIR}/config-${port}.yaml"
  unit="hysteria-server-${port}.service"
  if [ ! -f "/etc/systemd/system/${unit}" ]; then
    sed "s|/etc/hysteria/config.yaml|${HY2_DIR}/config-${port}.yaml|g; s|Hysteria2 Server|Hysteria2 Server (UDP ${port})|g" \
      /etc/systemd/system/hysteria-server.service >"/etc/systemd/system/${unit}"
  fi
  systemctl daemon-reload
  systemctl enable "${unit}"
  systemctl restart "${unit}"
  systemctl is-active --quiet "${unit}"
done

echo "applied xray (tcp/${VPN_SERVER_PORT:-443}) and hysteria2 (udp/${VPN_HY2_PORT:-443})"
