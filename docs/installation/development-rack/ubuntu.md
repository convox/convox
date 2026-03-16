---
title: "Ubuntu / Linux"
slug: ubuntu
url: /installation/development-rack/ubuntu
---
# Ubuntu / Linux

## Prerequisites

Install the following:
- [Convox CLI](/installation/cli)
- [Docker Engine](https://docs.docker.com/engine/install/ubuntu/)
- [Minikube](https://minikube.sigs.k8s.io/docs/start/)
- [Terraform](https://developer.hashicorp.com/terraform/install)

These instructions work on Ubuntu 22.04+ and other Linux distributions including WSL2.

## Installation

### 1. Choose a rack name

Pick a name for your local rack. This name becomes part of the domain used to reach your applications. We will use `dev` throughout this guide.

### 2. Start Minikube

Start Minikube with the Docker driver, the insecure registry flag for your rack name, and the required static IP:

```bash
minikube start \
  --driver=docker \
  --kubernetes-version=v1.33.0 \
  --insecure-registry="registry.dev.localdev.convox.cloud" \
  --static-ip=192.168.212.2
```

Replace `dev` with your chosen rack name in the `--insecure-registry` flag.

> The `--static-ip=192.168.212.2` is required because `*.localdev.convox.cloud` resolves to this address.

> Check the [Convox releases](https://github.com/convox/convox/releases) page to find the Kubernetes version for the rack version you want to install.

**Troubleshooting:**
- If you get an error about IP `192.168.212.2`, run `docker network prune` and try again.
- If you get a Docker permissions error, follow the instructions in the error output to add your user to the `docker` group.

### 3. Enable required addons

```bash
minikube addons enable ingress
minikube addons enable ingress-dns
```

### 4. Install the rack

```bash
convox rack install local dev -v 3.23.3
```

Replace `3.23.3` with the latest version from the [releases page](https://github.com/convox/convox/releases).

### 5. Verify the installation

```bash
convox rack -r dev
```

You should see output like:

```text
Name      dev
Provider  local
Router    router.dev.localdev.convox.cloud
Status    running
Version   3.23.3
```

## Using the rack

Switch to your new rack:

```bash
convox switch dev
```

Now all `convox` commands will target your local rack. You can deploy apps with `convox deploy`, start local development with `convox start`, and use all the same commands you would on a production rack.

Your applications will be available at `https://<service>.<app>.dev.localdev.convox.cloud`.

> Browsers will show a certificate warning because the local rack uses self-signed TLS certificates. This is expected.

## Management

### Rack files

Local rack configuration is stored at:

```text
~/.config/convox/racks/<RACK_NAME>
```

### Stopping and starting

```bash
minikube stop    # pause the cluster (preserves state)
minikube start   # resume the cluster
```

### Uninstalling

To uninstall the rack cleanly:

```bash
convox rack uninstall -r <RACK_NAME>
```

If that fails, you can remove the rack files manually:

```bash
rm -rf ~/.config/convox/racks/<RACK_NAME>
minikube delete
```

## See Also

- [Running Locally](/development/running-locally) for using `convox start` to develop locally
- [Local Development Tutorial](/tutorials/local-development) for a guided walkthrough
