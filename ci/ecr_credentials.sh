#!/bin/bash
set -eo pipefail

# no xtrace: the login password must not reach the log
# the repository is kept out of us-east-1, where the convox/rack CI reaper deletes every repository
region=us-east-2
account=$(aws sts get-caller-identity --query Account --output text)
password=$(aws ecr get-login-password --region "${region}")
echo "::add-mask::${password}"

{
  echo "ECR_REGISTRY_HOST=${account}.dkr.ecr.${region}.amazonaws.com"
  echo "ECR_REGISTRY_PASSWORD=${password}"
} >> "$GITHUB_ENV"
