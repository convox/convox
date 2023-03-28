#!/bin/bash
set -ex -o pipefail

if [ $VERSION == "" ]; then
  convox rack install ${PROVIDER} ${RACK_NAME} region=${REGION} ${RACK_PARAMS}
else
  convox rack install ${PROVIDER} ${RACK_NAME} -v ${VERSION} region=${REGION} ${RACK_PARAMS}
fi
