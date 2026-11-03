#!/bin/bash
set -x -o pipefail

. "$(dirname "$0")/lib.sh"

if ! command -v convox >/dev/null; then
  echo "convox is not installed, nothing to uninstall"
  exit 0
fi

for i in 1 2 3; do
  convox rack uninstall ${RACK_NAME} 2>&1 | redact && exit 0
  [ "$i" -lt 3 ] && sleep $((i * 60))
done
exit 1
