---
title: "macOS-M1"
draft: false
slug: macOS-M1
url: /installation/development-rack/macos-m1
---
# macOS (M1/ARM64)

## Initial Setup

To use Convox’s local development Rack feature on macOS, you will need install following applications locally:
- [Convox](/reference/primitives/getting-started/introduction/#install-the-convox-cli-and-login)
- [Docker Desktop](https://docs.docker.com/desktop/install/mac-install/)
- [Minikube 1.29.0 or later](https://minikube.sigs.k8s.io/docs/start/)
- [Terraform](https://developer.hashicorp.com/terraform/downloads)


## Installation

Convox development racks run on Minikube with several required options:
- `--insecure-registry=”registry.<RACK_NAME>.macdev.convox.cloud”`
- `--kubernetes-version=<k8s_VERSION>`
- `--driver=docker`

The first thing you must choose is the name for your rack as it will be a component of other installation parameters.  For this documentation we will use the rack name: `test-local`

Then verify the Kubernetes version for the [rack version](https://github.com/convox/convox/releases) you are going to install.  At the time of this writing it is `v1.23.0`

You will start Minikube with the command:
`minikube start --kubernetes-version=<k8s_VERSION> --insecure-registry="https://registry.<RACK_NAME>.macdev.convox.cloud" --driver=docker`

For this example the command would be:
`minikube start --kubernetes-version=v1.23.0 --insecure-registry="https://registry.test-local.macdev.convox.cloud" --driver=docker`
- If you receive an error regarding docker permissions, you can run the group command given in the output.

Now enable Minikube ingress and ingress-dns addons:
` minikube addons enable ingress`
` minikube addons enable ingress-dns`


Then install the local development rack with the command `convox rack install local <RACK_NAME> -v <VERSION> os=mac`
> **_NOTE:_**: Rack version `-v` must be `3.10.5` or later

For this example the command would be:
`convox rack install local test-local -v 3.10.5 os=mac`

Once the rack is installed you should be able to see it by running: `convox racks`

Finally, to address the rack on macOS you must open a new terminal window and run `minikube tunnel`
> **_NOTE:_**: You must keep the terminal window with the tunnel active open to address the rack

You can now run: `convox rack -r <RACK_NAME>` to verify connectivity.


## Management

Local rack files are stored at `/Users/$USER/Library/Preferences/convox/racks/<RACK_NAME>`
If you run into any issues installing and need to delete the local rack you can run:
`sudo rm –rf /Users/$USER/Library/Preferences/convox/racks/<RACK_NAME>`