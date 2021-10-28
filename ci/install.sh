#!/bin/bash
set -ex -o pipefail

convox rack install ${PROVIDER} ${RACK_NAME} -v ${VERSION} ${RACK_PARAMS}
