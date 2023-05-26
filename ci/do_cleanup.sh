#!/bin/bash

# install doctl
wget https://github.com/digitalocean/doctl/releases/download/v1.94.0/doctl-1.94.0-linux-amd64.tar.gz
tar xf ./doctl-1.94.0-linux-amd64.tar.gz
sudo mv ./doctl /usr/bin

# Set your DigitalOcean access token
ACCESS_TOKEN=${DIGITALOCEAN_TOKEN}

export AWS_ACCESS_KEY_ID=${DIGITALOCEAN_ACCESS_ID}
export AWS_SECRET_ACCESS_KEY=${DIGITALOCEAN_SECRET_KEY}

# List Kubernetes clusters
clusters=$(doctl kubernetes cluster list --format Name --no-header)
echo "Kubernetes Clusters:"
echo "$clusters"

# Delete Kubernetes clusters
while IFS= read -r cluster; do
    doctl kubernetes cluster delete "$cluster" -f --access-token "$ACCESS_TOKEN"
done <<< "$clusters"

# List Spaces buckets
buckets=$(aws s3 ls --endpoint=https://nyc3.digitaloceanspaces.com  | awk '{print $3}')
echo "Spaces Buckets:"
echo "$buckets"

# Delete Spaces buckets
while IFS= read -r bucket; do
    aws s3 rb s3://$bucket --force --endpoint=https://nyc3.digitaloceanspaces.com
done <<< "$buckets"


# List volumes
volumes=$(doctl compute volume list --format Name --no-header)
echo "Volumes:"
echo "$volumes"

# Delete volumes
while IFS= read -r volume; do
    doctl compute volume delete "$volume" -f --access-token "$ACCESS_TOKEN"
done <<< "$volumes"
