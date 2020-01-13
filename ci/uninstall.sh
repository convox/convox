#!/bin/bash
set -ex -o pipefail

cd install/${PROVIDER}

terraform destroy -var name=${RACK_NAME} -auto-approve -lock=false
