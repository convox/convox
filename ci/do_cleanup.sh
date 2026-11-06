#!/bin/bash

set -uo pipefail

. "$(dirname "$0")/lib.sh"

# install doctl
curl -fsSL https://github.com/digitalocean/doctl/releases/download/v1.186.0/doctl-1.186.0-linux-amd64.tar.gz -o /tmp/doctl.tar.gz || exit 1
echo "a36f2ae5e672160c3a20ab46e8cef79c9b24e8055417f4beb790a897c532d241  /tmp/doctl.tar.gz" | sha256sum -c - || exit 1
tar xf /tmp/doctl.tar.gz -C /tmp && sudo mv /tmp/doctl /usr/bin

for v in DIGITALOCEAN_TOKEN DIGITALOCEAN_ACCESS_ID DIGITALOCEAN_SECRET_KEY REGION; do
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
    ci_rack_is_old "$cluster" || { echo "skipping $cluster: younger than 8 hours"; continue; }
    doctl kubernetes cluster delete "$cluster" -f --dangerous || true
  done <<< "$clusters"
fi

buckets=$(aws s3 ls --endpoint-url=https://${REGION}.digitaloceanspaces.com | awk '{print $3}' | grep '^ci-' || true)
echo "Spaces Buckets:"
echo "$buckets"
if [ -n "$buckets" ]; then
  while IFS= read -r bucket; do
    [ -z "$bucket" ] && continue
    ci_rack_is_old "$bucket" || { echo "skipping $bucket: younger than 8 hours"; continue; }
    aws s3 rb "s3://$bucket" --force --endpoint-url=https://${REGION}.digitaloceanspaces.com || true
  done <<< "$buckets"
fi

cutoff=$(date -u -d '8 hours ago' +%Y-%m-%dT%H:%M:%SZ)
volumes=$(doctl compute volume list -o json | jq -r --arg c "$cutoff" '.[] | select((.droplet_ids // [] | length) == 0 and .created_at < $c) | .id' || true)
echo "Unattached volumes older than 8 hours:"
echo "$volumes"
if [ -n "$volumes" ]; then
  while IFS= read -r volume; do
    [ -z "$volume" ] && continue
    doctl compute volume delete "$volume" -f || true
  done <<< "$volumes"
fi
