---
title: "Digital Ocean"
slug: digital-ocean
url: /installation/production-rack/do
---
# Digital Ocean
> These are instructions for installing a Rack via the command line. The easiest way to install a Rack is with the [Convox Web Console](https://console.convox.com)

## Initial Setup

### Terraform

- Install [Terraform](https://learn.hashicorp.com/terraform/getting-started/install.html)

### Convox CLI

- [Install the Convox CLI](/installation/cli)

## Environment

The following environment variables are required:

- `DIGITALOCEAN_ACCESS_ID`
- `DIGITALOCEAN_SECRET_KEY`
- `DIGITALOCEAN_TOKEN`

### Create Token

Go to https://cloud.digitalocean.com/account/api/tokens and generate a new Personal Access Token.

- `DIGITALOCEAN_TOKEN` is the token you created

### Create Spaces Access Key

Go to https://cloud.digitalocean.com/account/api/tokens and generate a new Spaces Access Key.

- `DIGITALOCEAN_ACCESS_ID` is the resulting Key
- `DIGITALOCEAN_SECRET_KEY` is the Secret

## Install Rack
```bash
    $ convox rack install do <name> [param1=value1]...
```
### Available Parameters

| Name              | Default         | Description                                                             |
| ------------------| ----------------| ----------------------------------------------------------------------- |
| **cert_duration** | **2160h**       | Certification renew period                                              |
| **node_type**     | **s-2vcpu-4gb** | [Node instance type](https://slugs.do-api.dev/)                         |
| **region**        | **nyc3**        | [Digital Ocean region](https://slugs.do-api.dev/)                       |
| **registry_disk** | **50Gi**        | Registry disk size                                                      |
| **syslog**        |                 | Forward logs to a syslog endpoint (e.g. **tcp+tls://example.org:1234**) |

## Post-Installation

After the install completes, verify your rack is running:

```bash
    $ convox rack
    Name      myrack
    Provider  do
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running
    Version   3.23.3
```

Installation typically takes 10-20 minutes while DOKS provisions the cluster.

## See Also

- [CLI Rack Management](/management/cli-rack-management) for managing and updating your Rack
- [Deploying an Application](/tutorials/deploying-an-application) to deploy your first app
- [Rack Parameters: Digital Ocean](/configuration/rack-parameters/do) for a full list of configurable parameters
