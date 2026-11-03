#!/bin/bash
set -eo pipefail

# no xtrace: the login password must not reach the log
account=$(aws sts get-caller-identity --query Account --output text)
password=$(aws ecr get-login-password --region "${AWS_REGION}")
echo "::add-mask::${password}"

{
  echo "ECR_REGISTRY_HOST=${account}.dkr.ecr.${AWS_REGION}.amazonaws.com"
  echo "ECR_REGISTRY_PASSWORD=${password}"
} >> "$GITHUB_ENV"
