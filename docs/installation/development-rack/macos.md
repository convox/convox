---
title: "macOS"
draft: false
slug: macOS
url: /installation/development-rack/macos
---
# macOS

## Initial Setup

### Kubernetes

- Install [Docker Desktop](https://www.docker.com/products/docker-desktop)
- Go to Docker Desktop Preferences
- Go to the Advanced tab
- Drag the CPU slider to the halfway point
- Drag the Memory slider to at least 8GB
- Go to the Kubernetes tab
- Enable Kubernetes

### Terraform

- Install [Terraform](https://learn.hashicorp.com/terraform/getting-started/install.html)

### Convox CLI
```html
    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox
```
## Installation

> Make sure that your `kubectl` points at your local docker setup.  Ensure that if your `KUBECONFIG` env var is set, it is pointing at a config file that contains your desktop context.  By default, your local context will be installed into `~/.kube/config`.

Also ensure that the context is indeed set to `docker-desktop`. If not, just run:
```html
    $ kubectl config use-context docker-desktop
```
Install a local Rack named `dev`.
```html
    $ convox rack install local dev
```
## DNS Setup

Set `*.convox` to be resolved by the local Rack's DNS server.
```html
    $ sudo mkdir -p /etc/resolver
    $ sudo bash -c 'echo "nameserver 127.0.0.1" > /etc/resolver/convox'
```
## CA Trust

To remove browser warnings about untrusted certificates for local applications
you can trust the Rack's CA certificate.

This certificate is generated on your local machine and is unique to your Rack.
```html
    $ kubectl get secret/ca -n dev-system -o jsonpath="{.data.tls\.crt}" | base64 --decode > /tmp/ca
    $ sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain /tmp/ca
```