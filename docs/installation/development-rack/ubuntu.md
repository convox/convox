---
title: "Ubuntu"
draft: false
slug: Ubuntu
url: /installation/development-rack/ubuntu
---
# Ubuntu

## Initial Setup

> **_NOTE:_**:  These instructions are intended for Ubuntu 22.04 LTS.  While alternate versions may function with no issues, version 22.04 LTS is our baseline for updates and testing. 

To use Convox’s local development Rack feature on Ubuntu, you will need install following applications locally: 
- [Convox](/reference/primitives/getting-started/introduction/#install-the-convox-cli-and-login) 
- [Docker](https://docs.docker.com/engine/install/ubuntu/) 
- [Minikube 1.29.0 or later](https://minikube.sigs.k8s.io/docs/start/) 
- [Terraform](https://developer.hashicorp.com/terraform/downloads) 

## Installation

Convox development racks run on Minikube with several required options: 
- `--insecure-registry=”registry.<RACK_NAME>.localdev.convox.cloud”`
- `--kubernetes-version=<k8s_VERSION>` 
- `--static-ip <IP>` 

The first thing you must choose is the name for your rack as it will be a component of other installation parameters.  For this documentation we will use the rack name `test-local` 

Then verify the Kubernetes version for the [rack version](https://github.com/convox/convox/releases) you are going to install.  At the time of this writing it is `v1.23.0` 

Finally choose a static IP that works for your local configuration.  We have found `192.168.212.2` to work well in most all cases. 


Now start Minikube with: 
`minikube start --kubernetes-version=v1.23.0 --insecure-registry="https://registry.test-local.localdev.convox.cloud" --static-ip 192.168.212.2` 

- If you receive an error regarding IP `192.168.212.2`, you can delete the docker networks by running `docker network prune`, and then run the full `minikube start` command above again. 
- If you receive an error regarding docker permissions, you can run the group command given in the output. 


Now enable Minikube ingress and ingress-dns addons: 
` minikube addons enable ingress` 
` minikube addons enable ingress-dns`  

Finally install the local development rack with the command `convox rack install local <RACK_NAME> -v <VERSION>` 
> **_NOTE:_**: Rack version `-v` must be 3.10.5 or later 

For this example the command would be:
`convox rack install local test-local -v 3.10.5` 

You can now run `convox rack -r <RACK_NAME>` to see the new rack installed locally. 
 
## Management

Local rack files are stored at `~/.config/convox/racks/<RACK_NAME>` 
If you run into any issues installing and need to delete the local rack you can run: 
`sudo rm –rf ~/.config/convox/racks/<RACK_NAME>` 