#!/bin/bash
set -ex -o pipefail

if [[ ! -z NODE_TYPE ]]; then
    convox rack install ${PROVIDER} ${RACK_NAME} -v ${VERSION} region=${REGION} node_type=${NODE_TYPE}
else
    convox rack install ${PROVIDER} ${RACK_NAME} -v ${VERSION} region=${REGION}
fi

