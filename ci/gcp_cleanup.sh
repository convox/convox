#!/bin/bash

# install gcloud
sudo apt-get update

sudo apt-get install apt-transport-https ca-certificates gnupg curl sudo
echo "deb [signed-by=/usr/share/keyrings/cloud.google.asc] https://packages.cloud.google.com/apt cloud-sdk main" | sudo tee -a /etc/apt/sources.list.d/google-cloud-sdk.list
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key --keyring /usr/share/keyrings/cloud.google.gpg add -

sudo apt-get update && sudo apt-get install google-cloud-cli


# Set your gcp access
echo ${GOOGLE_CREDENTIALS} > ~/g-key.json

gcloud auth activate-service-account --key-file=~/g-key.json

gcloud config set project ${GOOGLE_PROJECT}

gcloud config set compute/region ${GOOGLE_REGION}+

clusters=$(gcloud container clusters list --format="value(name,zone)" --filter="name:ci*")

echo "$clusters" | while read -r name zone; do
  echo "Deleting GKE cluster: $name in zone $zone"
  gcloud container clusters delete "$name" --zone="$zone" --quiet
done
