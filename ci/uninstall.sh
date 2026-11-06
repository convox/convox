#!/bin/bash
set -x -o pipefail

. "$(dirname "$0")/lib.sh"

if ! command -v convox >/dev/null; then
  echo "convox is not installed, nothing to uninstall"
  exit 0
fi

# terraform reads the github api anonymously from this runner; on a 403 wait for the rate limit to reset
for i in 1 2 3; do
  convox rack uninstall ${RACK_NAME} 2>&1 | redact | tee /tmp/uninstall.out && exit 0
  [ "$i" -lt 3 ] || break
  wait=$((i * 60))
  if grep -q 'Response code: 403' /tmp/uninstall.out; then
    reset=$(curl -fsS --max-time 10 https://api.github.com/rate_limit | jq -r '.resources.core.reset // 0' || echo 0)
    wait=$((reset - $(date +%s) + 5))
    [ "$wait" -gt 60 ] || wait=60
    [ "$wait" -lt 3900 ] || wait=3900
  fi
  sleep $wait
done
exit 1
