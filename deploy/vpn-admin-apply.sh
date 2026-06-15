#!/usr/bin/env bash
set -euo pipefail

XRAY_CANDIDATE="${VPN_XRAY_CANDIDATE:-/var/lib/go-sqlite-api/xray-config.json}"
XRAY_CONFIG="/usr/local/etc/xray/config.json"

if [ ! -f "${XRAY_CANDIDATE}" ]; then
  echo "xray candidate config does not exist: ${XRAY_CANDIDATE}" >&2
  exit 2
fi

/usr/local/bin/xray run -test -config "${XRAY_CANDIDATE}"

install -o root -g root -m 644 "${XRAY_CANDIDATE}" "${XRAY_CONFIG}"

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

for unit in $(systemctl list-unit-files 'hysteria-server*.service' --no-legend 2>/dev/null | awk '{print $1}'); do
  systemctl disable --now "${unit}" 2>/dev/null || true
  rm -f "/etc/systemd/system/${unit}" 2>/dev/null || true
done
rm -f /etc/hysteria/config-*.yaml /var/lib/go-sqlite-api/hysteria2-config*.yaml 2>/dev/null || true
systemctl daemon-reload

echo "applied xray (tcp/${VPN_SERVER_PORT:-443}); hysteria2 disabled"
