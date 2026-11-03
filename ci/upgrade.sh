#!/bin/bash
set -ex -o pipefail

. "$(dirname "$0")/lib.sh"

convox rack update ${VERSION} -r ${RACK_NAME} 2>&1 | redact

convox switch ${RACK_NAME}

wait_ready

version=$(convox rack | grep Version | awk -F '  +' '{print $2}')
if [ "${version}" != "${VERSION}" ]; then
  exit 1;
fi
