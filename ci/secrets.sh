#!/bin/bash

export_secret() {
  if [ -z "${!1}" ]; then
    echo "missing secret: $1" >&2
    exit 1
  fi
  echo "${2:-$1}=${!1}" >> "$GITHUB_ENV"
}

case "$PROVIDER" in
aws)
  export_secret AWS_ACCESS_KEY_ID
  export_secret AWS_REGION AWS_DEFAULT_REGION # we also want to export AWS_DEFAULT_REGION for the aws cli config
  export_secret AWS_REGION REGION
  export_secret AWS_SECRET_ACCESS_KEY
  ;;
azure)
  export_secret ARM_CLIENT_ID
  export_secret ARM_CLIENT_SECRET
  export_secret ARM_REGION REGION
  export_secret ARM_SUBSCRIPTION_ID
  export_secret ARM_TENANT_ID
  ;;
do)
  export_secret DIGITALOCEAN_ACCESS_ID
  export_secret DIGITALOCEAN_REGION REGION
  export_secret DIGITALOCEAN_SECRET_KEY
  export_secret DIGITALOCEAN_TOKEN
  ;;
gcp)
  export_secret GOOGLE_CREDENTIALS
  export_secret GOOGLE_PROJECT
  export_secret GOOGLE_REGION
  export_secret GOOGLE_REGION REGION
  ;;
esac
