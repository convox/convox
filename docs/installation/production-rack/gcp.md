---
title: "Google Cloud"
slug: google-cloud
url: /installation/production-rack/gcp
---
# Google Cloud
> These are instructions for installing a Rack via the command line. The recommended way to install a Rack is with the [Convox Web Console](https://console.convox.com)

## Initial Setup

### Google Cloud CLI

- [Install the Google Cloud CLI](https://cloud.google.com/sdk/docs/#install_the_latest_cloud_tools_version_cloudsdk_current_version)
- Run `gcloud init`

### Terraform

- Install [Terraform](https://developer.hashicorp.com/terraform/install)

### Convox CLI

- [Install the Convox CLI](/installation/cli)

## Environment

The following environment variables are required:

- `GOOGLE_CREDENTIALS`
- `GOOGLE_PROJECT`

### Create Project
```bash
    $ gcloud projects create <id> --set-as-default
```
- `GOOGLE_PROJECT` is the id you selected

> You will need to enable billing on this new project before proceeding. Visit https://console.cloud.google.com/billing to set up billing for your project.

### Create Service Account
```bash
    $ serviceaccount="convox@${GOOGLE_PROJECT}.iam.gserviceaccount.com"
    $ gcloud iam service-accounts create convox
    $ gcloud iam service-accounts keys create ~/.gcloud.convox --iam-account="${serviceaccount}"
```
- `GOOGLE_CREDENTIALS` is `~/gcloud.convox`

### Grant Permissions
```bash
    $ gcloud projects add-iam-policy-binding ${GOOGLE_PROJECT} --member="serviceAccount:${serviceaccount}" --role="roles/owner"
```

## Enable GCP APIs

The following APIs must be enabled for your GCP project:

```bash
    $ gcloud services enable cloudapis.googleapis.com
    $ gcloud services enable compute.googleapis.com
    $ gcloud services enable cloudresourcemanager.googleapis.com
    $ gcloud services enable container.googleapis.com
    $ gcloud services enable serviceusage.googleapis.com
    $ gcloud services enable servicemanagement.googleapis.com
```

## Install Rack
```bash
    $ convox rack install gcp <name> [param1=value1]...
```
### Available Parameters

| Name          | Default         | Description                                                                              |
| ------------- | --------------- | ---------------------------------------------------------------------------------------- |
| **cert_duration** | **2160h**         | Certificate renewal period                                                                 |
| **node_type**     | **n1-standard-1** | Node instance type                                                                         |
| **preemptible**   | **true**          | Use [preemptible](https://cloud.google.com/compute/docs/instances/preemptible) instances   |
| **region**        | **us-east1**      | GCP Region                                                                                 |
| **syslog**        |                   | Forward logs to a syslog endpoint (e.g. **tcp+tls://example.org:1234**)                    |

## Post-Installation

After the install completes, verify your rack is running:

```bash
    $ convox rack
    Name      myrack
    Provider  gcp
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running
    Version   3.23.3
```

Installation typically takes 10-20 minutes while GKE provisions the cluster and node pools.

## See Also

- [CLI Rack Management](/management/cli-rack-management) for managing and updating your Rack
- [Deploying an Application](/tutorials/deploying-an-application) to deploy your first app
- [Rack Parameters: GCP](/configuration/rack-parameters/gcp) for a full list of configurable parameters