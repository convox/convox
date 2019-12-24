# Convox on Local Workstation

## Initial Setup

### MacOS

- Install [Docker Desktop](https://www.docker.com/products/docker-desktop)
- Go to Docker Desktop Preferences
- Go to the Advanced tab
- Drag the CPU slider to the halfway point
- Drag the Memory slider to at least 8GB
- Go to the Kubernetes tab
- Enable Kubernetes

### Ubuntu

- `snap install microk8s --classic --channel=1.13/stable`
- `microk8s.enable dns storage`

## Install Convox

- Clone this repository and switch to the directory containing this `README`
- Run `terraform init`
- Run `terraform apply`

## DNS Setup

Set up `*.convox` to be resolved by the local Rack's DNS server

### MacOS

- `sudo mkdir -p /etc/resolver`
- `sudo bash -c 'echo "nameserver 127.0.0.1" > /etc/resolver/convox'`

### Linux

- TBD

## Convox CLI Setup

- [Install the Convox CLI](../../docs/guides/installation/cli.md)
- Run `export RACK_URL=$(terraform output rack_url)`
- Run `convox rack` to ensure that your CLI is connected to your new Rack