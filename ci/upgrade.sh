#!/bin/bash
set -ex -o pipefail

convox rack update ${VERSION} -r ${RACK_NAME}

sleep 30

convox switch ${RACK_NAME}

convox rack | grep 'Status' | xargs | cut -d ' ' -f2 | grep "running"
