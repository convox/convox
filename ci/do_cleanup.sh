#!/bin/bash

set -uo pipefail

# install doctl
wget https://github.com/digitalocean/doctl/releases/download/v1.94.0/doctl-1.94.0-linux-amd64.tar.gz
tar xf ./doctl-1.94.0-linux-amd64.tar.gz
sudo mv ./doctl /usr/bin

for v in DIGITALOCEAN_TOKEN DIGITALOCEAN_ACCESS_ID DIGITALOCEAN_SECRET_KEY; do
  val="${!v:-}"
  if [ -z "$val" ] || [ "$val" = "null" ]; then
    echo "$v is not set" >&2
    exit 1
  fi
done

# doctl authenticates with the DO API token. Spaces is S3-compatible, so the aws s3 calls below
# reach it via the AWS_* env vars (the only creds the aws CLI reads) set to the DO Spaces keys.
export DIGITALOCEAN_ACCESS_TOKEN="${DIGITALOCEAN_TOKEN}"
export AWS_ACCESS_KEY_ID="${DIGITALOCEAN_ACCESS_ID}"
export AWS_SECRET_ACCESS_KEY="${DIGITALOCEAN_SECRET_KEY}"

clusters=$(doctl kubernetes cluster list --format Name --no-header | grep '^ci-' || true)
echo "Kubernetes Clusters:"
echo "$clusters"
if [ -n "$clusters" ]; then
  while IFS= read -r cluster; do
    [ -z "$cluster" ] && continue
    doctl kubernetes cluster delete "$cluster" -f || true
  done <<< "$clusters"
fi

buckets=$(aws s3 ls --endpoint-url=https://nyc3.digitaloceanspaces.com | awk '{print $3}' || true)
echo "Spaces Buckets:"
echo "$buckets"
if [ -n "$buckets" ]; then
  while IFS= read -r bucket; do
    [ -z "$bucket" ] && continue
    aws s3 rb "s3://$bucket" --force --endpoint-url=https://nyc3.digitaloceanspaces.com || true
  done <<< "$buckets"
fi

volumes=$(doctl compute volume list --format Name,DropletIDs --no-header || true)
echo "Volumes:"
echo "$volumes"
if [ -n "$volumes" ]; then
  while read -r volume droplets; do
    [ -z "$volume" ] && continue
    [ -n "$droplets" ] && continue
    doctl compute volume delete "$volume" -f || true
  done <<< "$volumes"
fi
