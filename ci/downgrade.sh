#!/bin/bash
set -ex -o pipefail

export LATEST_RELEASE=$(curl -s https://api.github.com/repos/convox/convox/releases/latest | jq -r '.tag_name')

convox rack update ${LATEST_RELEASE} -r ${RACK_NAME}

sleep 30

convox switch ${RACK_NAME}

status=$(convox rack | grep 'Status' | xargs | cut -d ' ' -f2)

while [ "${status}" != "running" ]
do
    echo "waiting for rack updates to complete..."
    sleep 30
    status=$(convox rack | grep 'Status' | xargs | cut -d ' ' -f2)
    echo "rack status: ${status}"
    if [ "${status}" = "failed" ]; then
        exit 1
    fi
done
