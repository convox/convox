#!/bin/bash
set -ex -o pipefail

cd install/${PROVIDER}

terraform init
terraform apply -var name=${RACK_NAME} -var release=${VERSION} -auto-approve