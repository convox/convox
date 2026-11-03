#!/bin/bash
set -ex -o pipefail

. "$(dirname "$0")/lib.sh"

if [ -z "${VERSION}" ]; then
  convox rack install ${PROVIDER} ${RACK_NAME} region=${REGION} ${RACK_PARAMS} 2>&1 | redact
else
  convox rack install ${PROVIDER} ${RACK_NAME} -v ${VERSION} region=${REGION} ${RACK_PARAMS} 2>&1 | redact
fi
