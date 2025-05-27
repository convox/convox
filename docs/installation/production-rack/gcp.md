---
title: "Google Cloud"
draft: false
slug: Google Cloud
url: /installation/production-rack/gcp
---
# Google Cloud
> Please note that these are instructions for installing a Rack via the command line. The easiest way to install a Rack is with the [Convox Web Console](https://console.convox.com)

## Initial Setup

### Google Cloud CLI

- [Install the Google Cloud CLI](https://cloud.google.com/sdk/docs/#install_the_latest_cloud_tools_version_cloudsdk_current_version)
- Run `gcloud init`

### Terraform

- Install [Terraform](https://learn.hashicorp.com/terraform/getting-started/install.html)

### Convox CLI

- [Install the Convox CLI](/installation/cli)

## Environment

The following environment variables are required:

- `GOOGLE_CREDENTIALS`
- `GOOGLE_PROJECT`

### Create Project
```html
    $ gcloud projects create <id> --set-as-default
```
- `GOOGLE_PROJECT` is the id you selected

> You will need to enable billing on this new project before proceeding. Visit https://console.cloud.google.com/billing to set up billing for your project.

### Create Service Account
```html
    $ serviceaccount="convox@${GOOGLE_PROJECT}.iam.gserviceaccount.com"
    $ gcloud iam service-accounts create convox
    $ gcloud iam service-accounts keys create ~/.gcloud.convox --iam-account="${serviceaccount}"
```
- `GOOGLE_CREDENTIALS` is `~/gcloud.convox`

### Grant Permissions
```html
    $ gcloud projects add-iam-policy-binding ${GOOGLE_PROJECT} --member="serviceAccount:${serviceaccount}" --role="roles/owner"
```

## Enable GCP APIs

The following APIs must be enabled for your GCP project:

```html
    $ gcloud services enable cloudapis.googleapis.com
    $ gcloud services enable compute.googleapis.com
    $ gcloud services enable cloudresourcemanager.googleapis.com
    $ gcloud services enable container.googleapis.com
    $ gcloud services enable serviceusage.googleapis.com
    $ gcloud services enable servicemanagement.googleapis.com
```

## Install Rack
```html
    $ convox rack install gcp <name> [param1=value1]...
```
### Available Parameters

| Name          | Default         | Description                                                                              |
| ------------- | --------------- | ---------------------------------------------------------------------------------------- |
| **cert_duration** | **2160h**         | Certification renew period                                                                 |
| **node_type**     | **n1-standard-1** | Node instance type                                                                         |
| **preemptible**   | **true**          | Use [preemptible](https://cloud.google.com/compute/docs/instances/preemptible) instances   |
| **region**        | **us-east1**      | GCP Region                                                                                 |
| **syslog**        |                   | Forward logs to a syslog endpoint (e.g. **tcp+tls://example.org:1234**)                    |