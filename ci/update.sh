#!/bin/bash
set -ex -o pipefail

convox rack update ${VERSION} -r ${RACK_NAME}
