#!/bin/bash
set -ex -o pipefail

# install utilities
sudo apt-get update && sudo apt-get -y install jq unzip

# install docker
curl -s https://download.docker.com/linux/static/stable/x86_64/docker-29.5.2.tgz | sudo tar -C /usr/bin --strip-components 1 -xz

# install kubectl (match the rack's EKS k8s_version)
curl -Ls https://dl.k8s.io/release/v1.34.8/bin/linux/amd64/kubectl -o /tmp/kubectl && \
	sudo mv /tmp/kubectl /usr/bin/kubectl && sudo chmod +x /usr/bin/kubectl

# install terraform (1.x required by the hashicorp/aws 5.x provider the modules pin)
curl -L https://releases.hashicorp.com/terraform/1.15.5/terraform_1.15.5_linux_amd64.zip -o terraform.zip && \
	unzip terraform.zip -d /tmp && sudo mv /tmp/terraform /usr/bin/terraform && rm terraform.zip

# install latest aws cli
pip install --upgrade awscli
