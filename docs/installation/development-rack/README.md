---
title: "Development Rack"
slug: development-rack
url: /installation/development-rack
---
# Development Rack

A development Rack runs on your local workstation using [Minikube](https://minikube.sigs.k8s.io/) and allows you to work on your [App](/reference/primitives/app) in an environment nearly identical to production.

## Prerequisites

All platforms require the following:

- [Convox CLI](/installation/cli)
- [Docker](https://docs.docker.com/engine/install/)
- [Minikube](https://minikube.sigs.k8s.io/docs/start/)
- [Terraform](https://developer.hashicorp.com/terraform/install)

## Installation

Choose your local workstation's operating system to view its specific installation steps.
- [macOS (Intel and Apple Silicon)](/installation/development-rack/macos)
- [Ubuntu / Linux](/installation/development-rack/ubuntu)

## How It Works

The development Rack runs on a Minikube Kubernetes cluster on your machine. When you install a local Rack, Convox uses Terraform to provision the following inside Minikube:

- **API server** - the Convox control plane that handles all CLI commands
- **Registry** - a private Docker registry for storing your application images
- **Resolver** - a DNS server for routing `*.localdev.convox.cloud` (or `*.macdev.convox.cloud` on macOS) traffic within the cluster
- **Router** - routes external traffic through the Minikube ingress controller to your applications
- **cert-manager** - handles TLS certificates (self-signed for local development)

## See Also

- [Running Locally](/development/running-locally) for using `convox start` with a development Rack
- [Local Development Tutorial](/tutorials/local-development) for a guided walkthrough
