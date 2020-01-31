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

### Convox CLI

    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox

## Installation

Install a local Rack named `dev`.

    $ convox rack install local dev

## DNS Setup

Set `*.convox` to be resolved by the local Rack's DNS server.

    $ sudo mkdir -p /etc/resolver
    $ sudo bash -c 'echo "nameserver 127.0.0.1" > /etc/resolver/convox'

## CA Trust (optional)

To remove browser warnings about untrusted certificates for local applications
you can trust the Rack's CA certificate.

This certificate is generated on your local machine and is unique to your Rack.

    $ kubectl get secret/ca -n convox-system -o jsonpath="{.data.tls\.crt}" | base64 -d > /tmp/ca
    $ sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain /tmp/ca