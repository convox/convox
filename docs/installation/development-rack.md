# Development Rack

## Install Kubernetes

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
- `mkdir -p ~/.kube`
- `microk8s.config > ~/.kube/config`
- `sudo snap restart microk8s`

## Install Convox

- Create a new directory, e.g. `~/racks/development`
- Switch to this directory
- Create a file named `main.tf` with the following contents:

```
module "system" {
  source = "github.com/convox/convox//terraform/system/local"
  name   = "development"
}
```

- Run `terraform init`
- Run `terraform apply`

## DNS Setup

Set `*.convox` to be resolved by the local Rack's DNS server.

### MacOS

- `sudo mkdir -p /etc/resolver`
- `sudo bash -c 'echo "nameserver 127.0.0.1" > /etc/resolver/convox'`

### Ubuntu

- `sudo mkdir -p /usr/lib/systemd/resolved.conf.d`
- `sudo bash -c "printf '[Resolve]\nDNS=$(kubectl get service/resolver-external -n convox-system -o jsonpath="{.spec.clusterIP}")\nDomains=~convox' > /usr/lib/systemd/resolved.conf.d/convox.conf"`
- `systemctl daemon-reload`
- `systemctl restart systemd-networkd systemd-resolved`

## CA Trust (optional)

To remove browser warnings about untrusted certificates for local applications
you can trust the Rack's CA certificate.

This certificate is generated on your local machine and is unique to your Rack.

### MacOS

- `kubectl get secret/ca -n convox-system -o jsonpath="{.data.tls\.crt}" | base64 -d > /tmp/ca`
- `sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain /tmp/ca`

### Ubuntu

- `kubectl get secret/ca -n convox-system -o jsonpath="{.data.tls\.crt}" | base64 -d > /tmp/ca`
- `sudo mv /tmp/ca /usr/local/share/ca-certificates/convox.crt`
- `sudo update-ca-certificates`