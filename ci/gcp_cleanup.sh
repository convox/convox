#!/bin/bash

set -e

# install gcloud
# sudo apt-get update

# sudo apt-get install apt-transport-https ca-certificates gnupg curl sudo
# echo "deb [signed-by=/usr/share/keyrings/cloud.google.asc] https://packages.cloud.google.com/apt cloud-sdk main" | sudo tee -a /etc/apt/sources.list.d/google-cloud-sdk.list
# curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key --keyring /usr/share/keyrings/cloud.google.gpg add -

# sudo apt-get update && sudo apt-get install google-cloud-cli


# Set your gcp access
echo ${GOOGLE_CREDENTIALS} > /tmp/g-key.json

gcloud auth activate-service-account --key-file=/tmp/g-key.json

gcloud config set project ${GOOGLE_PROJECT}

gcloud config set compute/region ${GOOGLE_REGION}

clusters=$(gcloud container clusters list --format="value(name,location)" --filter="name ~ ^ci" || true)
echo "GKE clusters:"
echo "$clusters"
if [ -n "$clusters" ]; then
  while read -r name location; do
    [ -z "$name" ] && continue
    echo "Deleting GKE cluster: $name in $location"
    gcloud container clusters delete "$name" --location="$location" --quiet || true
  done <<< "$clusters"
fi

# CI-dedicated project, so unattached disks and reserved IPs are reaped project-wide.
# Convox GKE uses zonal persistent disks; -region:* keeps the reaper to disks the --zone delete handles.
disks=$(gcloud compute disks list --filter="-users:* AND -region:*" --format="value(name,zone.basename())" || true)
if [ -n "$disks" ]; then
  while read -r name zone; do
    [ -z "$name" ] && continue
    echo "Deleting orphaned disk: $name ($zone)"
    gcloud compute disks delete "$name" --zone="$zone" --quiet || true
  done <<< "$disks"
fi

addrs=$(gcloud compute addresses list --filter="status=RESERVED" --format="value(name,region.basename())" || true)
if [ -n "$addrs" ]; then
  while read -r name region; do
    [ -z "$name" ] && continue
    echo "Deleting reserved address: $name ($region)"
    if [ -n "$region" ]; then
      gcloud compute addresses delete "$name" --region="$region" --quiet || true
    else
      gcloud compute addresses delete "$name" --global --quiet || true
    fi
  done <<< "$addrs"
fi
