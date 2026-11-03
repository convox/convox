#!/bin/bash
set -ex -o pipefail

fetch_verified() {
  curl -fsSL "$1" -o "$2"
  echo "$3  $2" | sha256sum -c -
}

# install utilities
sudo apt-get update && sudo apt-get -y install jq unzip

# install docker
fetch_verified https://download.docker.com/linux/static/stable/x86_64/docker-29.5.2.tgz /tmp/docker.tgz 6d81a5be56232d9cc047e60b7110a087793536ae5a8b465719cd303f05fc56ec
sudo tar -C /usr/bin --strip-components 1 -xzf /tmp/docker.tgz

# install kubectl (match the rack's EKS k8s_version)
fetch_verified https://dl.k8s.io/release/v1.35.10/bin/linux/amd64/kubectl /tmp/kubectl c49a28f1f4efc4d475ed05f47f2ce8e563bf7bef69ed77fe1b924ea6d83b6327
sudo install -m 0755 /tmp/kubectl /usr/bin/kubectl

# install terraform (1.x required by the hashicorp/aws 5.x provider the modules pin)
fetch_verified https://releases.hashicorp.com/terraform/1.15.5/terraform_1.15.5_linux_amd64.zip /tmp/terraform.zip 702b2136af6728c8ff037f843dd2dbce2b7ad88786b7381d1d72aefa250f601c
unzip -o /tmp/terraform.zip terraform -d /tmp && sudo install -m 0755 /tmp/terraform /usr/bin/terraform
