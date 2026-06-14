#!/usr/bin/env bash
set -euo pipefail

FILE="${1:-/Users/melon/git_pro/aws_server/scripts/owner-clash-mac.yaml}"

if [[ ! -f "${FILE}" ]]; then
  echo "File not found: ${FILE}" >&2
  exit 1
fi

echo "Checking: ${FILE}"
echo

fail=0
check() {
  local label="$1"
  local pattern="$2"
  if grep -q "${pattern}" "${FILE}"; then
    echo "OK  ${label}"
    grep "${pattern}" "${FILE}" | head -1 | sed 's/^/    /'
  else
    echo "BAD ${label}"
    fail=1
  fi
}

check "uuid" "da289730-2524-44f4-8d76-d6a7af321084"
check "public-key" "SQZpcETHSkudvoGBuleYASszSLvJe9w7lpMR4U6ttGg"
check "server direct rule" "IP-CIDR,54.150.9.209/32,DIRECT"

if grep -q "vQ0xo0vNT19i4paArjTyW9dzWnIZCLJre5amEKqRqgk" "${FILE}"; then
  echo "BAD old public-key still present"
  fail=1
fi

if grep -q "IP-CIDR,54.150.9.209/32,PROXY" "${FILE}"; then
  echo "BAD server IP routed via PROXY (will break VPN)"
  fail=1
fi

echo
if [[ "${fail}" -eq 0 ]]; then
  echo "Config file looks correct."
else
  echo "Config file is wrong. Re-download owner Mac YAML."
  exit 1
fi
