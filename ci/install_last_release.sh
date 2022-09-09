#!/bin/bash
set -ex -o pipefail

export LATEST_RELEASE=$(curl -s https://api.github.com/repos/convox/convox/releases/latest | jq -r '.tag_name')

convox rack install ${PROVIDER} ${RACK_NAME} -v ${LATEST_RELEASE} region=${REGION} ${RACK_PARAMS}
