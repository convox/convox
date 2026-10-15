---
title: "Oracle Cloud"
description: "Install a Convox production Rack on Oracle Cloud Infrastructure via the command line, covering the API signing key, compartment, environment, and install parameters."
slug: oracle-cloud
url: /installation/production-rack/oci
---
# Oracle Cloud
> These are instructions for installing a Rack via the command line. The recommended way to install a Rack is with the [Convox Web Console](https://console.convox.com)

## Initial Setup

### OCI CLI

- [Install the OCI CLI](https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm)

> The `oci` CLI must stay installed and on `PATH` on any machine that runs `convox rack install oci` or manages the rack afterwards. OKE only issues exec-style kubeconfigs, so every `kubectl`/Terraform connection to the cluster shells out to `oci ce cluster generate-token` to fetch a token. There is no other way to authenticate to the cluster.

### Terraform

- Install [Terraform](https://developer.hashicorp.com/terraform/install)

### Convox CLI

- [Install the Convox CLI](/installation/cli)

## Environment

The following environment variables are required:

- `OCI_TENANCY_OCID`
- `OCI_USER_OCID`
- `OCI_FINGERPRINT`
- `OCI_PRIVATE_KEY`
- `OCI_REGION`

`OCI_COMPARTMENT_OCID` is optional and defaults to the tenancy root compartment.

### Find your Tenancy and User OCIDs

In the OCI Console, click your profile icon:

- **Tenancy: `<name>`** shows your tenancy OCID. `OCI_TENANCY_OCID` is this value.
- **User settings** shows your user OCID. `OCI_USER_OCID` is this value.

### Create an API Signing Key

In the OCI Console, go to **User settings > API keys > Add API key**, choose **Generate API Key Pair**, and download the private key.

- `OCI_FINGERPRINT` is the fingerprint shown after the key is added
- `OCI_PRIVATE_KEY` is the contents of the downloaded PEM file, not a path to it:
  ```bash
  $ export OCI_PRIVATE_KEY="$(cat ~/Downloads/oci_api_key.pem)"
  ```

### Set Region and Compartment

- `OCI_REGION` is the region you want the rack deployed to, e.g. `us-ashburn-1`
- `OCI_COMPARTMENT_OCID` is the OCID of the compartment to create rack resources in. It defaults to your tenancy root compartment if left unset.

### Permissions

The user creating the rack needs permission to manage IAM resources in the tenancy (the rack creates its own IAM user, group, policy, and auth token for pushing images to OCIR) and to manage compute, network, and container engine resources in the target compartment.

## Install Rack
```bash
$ convox rack install oci <name> [param1=value1]...
```
### Available Parameters

| Name              | Default                | Description                                                              |
| ------------------| ------------------------| ------------------------------------------------------------------------ |
| **cert_duration** | **2160h**               | Certificate renewal period                                               |
| **node_type**     | **VM.Standard.E4.Flex** | [Node compute shape](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm) |
| **node_ocpus**    | **2**                   | OCPUs per node (Flex shapes only)                                        |
| **node_memory**   | **16**                  | Memory per node in GB (Flex shapes only)                                 |
| **node_disk**     | **100**                 | Node boot volume size in GB                                              |
| **node_count**    | **2**                   | Number of nodes in the rack's node pool                                  |
| **region**        | **us-ashburn-1**        | [OCI region](https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm) |
| **gpu_node_type** |                         | GPU compute shape for an optional tainted GPU node pool                  |
| **syslog**        |                         | Forward logs to a syslog endpoint (e.g. **tcp+tls://example.org:1234**)  |

See [Rack Parameters: Oracle Cloud](/configuration/rack-parameters/oci) for the full list, including GPU node pool parameters.

## Post-Installation

After the install completes, verify your rack is running:

```bash
$ convox rack
Name      myrack
Provider  oci
Router    router.0a1b2c3d4e5f.convox.cloud
Status    running
Version   3.24.11
```

Installation typically takes 10-20 minutes while OKE provisions the cluster.

## See Also

- [CLI Rack Management](/management/cli-rack-management) for managing and updating your Rack
- [Deploying an Application](/tutorials/deploying-an-application) to deploy your first app
- [Rack Parameters: Oracle Cloud](/configuration/rack-parameters/oci) for a full list of configurable parameters
