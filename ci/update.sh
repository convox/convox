#!/bin/bash
set -ex -o pipefail

convox rack update ${VERSION} -r ${RACK_NAME}

sleep 15

convox rack | grep 'Status' | xargs | cut -d ' ' -f2 | grep "running"

version=$(convox rack | grep Version | awk -F '  +' '{print $2}')
if [ "${version}" != "${VERSION}" ]; then
  exit 1;
fi

param_version=$(convox rack params | grep release | awk -F '  +' '{print $2}')
if [ "${param_version}" != "${VERSION}" ]; then
  exit 1;
fi
