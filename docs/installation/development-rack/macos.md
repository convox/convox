---
title: "macOS"
slug: macos
url: /installation/development-rack/macos
---
# macOS

These instructions apply to both Intel (x86_64) and Apple Silicon (M1/M2/M3/ARM64) Macs.

## Prerequisites

Install the following:
- [Convox CLI](/installation/cli)
- [Docker Desktop](https://docs.docker.com/desktop/install/mac-install/)
- [Minikube](https://minikube.sigs.k8s.io/docs/start/)
- [Terraform](https://developer.hashicorp.com/terraform/install)

## Installation

### 1. Choose a rack name

Pick a name for your local rack. This name becomes part of the domain used to reach your applications. We will use `dev` throughout this guide.

### 2. Start Minikube

Start Minikube with the Docker driver and the insecure registry flag for your rack name:

```bash
minikube start \
  --driver=docker \
  --kubernetes-version=v1.33.0 \
  --insecure-registry="registry.dev.macdev.convox.cloud"
```

Replace `dev` with your chosen rack name in the `--insecure-registry` flag.

> Check the [Convox releases](https://github.com/convox/convox/releases) page to find the Kubernetes version for the rack version you want to install.

### 3. Enable required addons

```bash
minikube addons enable ingress
minikube addons enable ingress-dns
```

### 4. Install the rack

```bash
convox rack install local dev -v 3.23.3 os=mac
```

Replace `3.23.3` with the latest version from the [releases page](https://github.com/convox/convox/releases).

> The `os=mac` parameter is required on macOS. It configures the rack to use `*.macdev.convox.cloud` instead of `*.localdev.convox.cloud`.

### 5. Start the Minikube tunnel

Open a separate terminal window and run:

```bash
minikube tunnel
```

> You must keep this terminal open while using the rack. The tunnel allows your Mac to reach services inside the Minikube cluster.

### 6. Verify the installation

```bash
convox rack -r dev
```

You should see output like:

```text
Name      dev
Provider  local
Router    router.dev.macdev.convox.cloud
Status    running
Version   3.23.3
```

## Using the rack

Switch to your new rack:

```bash
convox switch dev
```

Now all `convox` commands will target your local rack. You can deploy apps with `convox deploy`, start local development with `convox start`, and use all the same commands you would on a production rack.

Your applications will be available at `https://<service>.<app>.dev.macdev.convox.cloud`.

> Browsers will show a certificate warning because the local rack uses self-signed TLS certificates. This is expected.

## Management

### Rack files

Local rack configuration is stored at:

```text
~/Library/Preferences/convox/racks/<RACK_NAME>
```

### Stopping and starting

```bash
minikube stop    # pause the cluster (preserves state)
minikube start   # resume the cluster
minikube tunnel  # remember to restart the tunnel after starting
```

### Uninstalling

To uninstall the rack cleanly:

```bash
convox rack uninstall -r <RACK_NAME>
```

If that fails, you can remove the rack files manually:

```bash
rm -rf ~/Library/Preferences/convox/racks/<RACK_NAME>
minikube delete
```

## See Also

- [Running Locally](/development/running-locally) for using `convox start` to develop locally
- [Local Development Tutorial](/tutorials/local-development) for a guided walkthrough
