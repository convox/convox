#!/bin/bash
set -ex -o pipefail

. "$(dirname "$0")/lib.sh"

base=${BASE_VERSION:-}
if [ -z "$base" ] && [ -n "${VERSION:-}" ]; then
  base=$(previous_release ${VERSION})
elif [ -z "$base" ]; then
  base=$(latest_release)
fi
if [ -z "$base" ]; then
  echo "no published release before ${VERSION}" >&2
  exit 1
fi

convox rack install ${PROVIDER} ${RACK_NAME} -v ${base} region=${REGION} ${RACK_PARAMS} 2>&1 | redact
