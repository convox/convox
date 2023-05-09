#!/bin/bash
set -ex -o pipefail

export LATEST_RELEASE=$(curl -s https://api.github.com/repos/convox/convox/releases/latest | jq -r '.tag_name')

convox rack update ${LATEST_RELEASE} -r ${RACK_NAME}

sleep 30

convox switch ${RACK_NAME}

convox rack | grep 'Status' | xargs | cut -d ' ' -f2 | grep "running"

version=$(convox rack | grep Version | awk -F '  +' '{print $2}')
if [ "${version}" != "${LATEST_RELEASE}" ]; then
  exit 1;
fi
