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

> You will likely need to set up Billing on this new project at https://console.cloud.google.com/billing before proceeding
> You can find the full documentation on this here: https://cloud.google.com/billing/docs/how-to/modify-project#enable_billing_for_an_existing_project

### Create Service Account
```html
    $ serviceaccount=$(gcloud iam service-accounts create convox --format="value(email)")
    $ gcloud iam service-accounts keys create ~/gcloud.convox --iam-account=${serviceaccount}
```
- `GOOGLE_CREDENTIALS` is `~/gcloud.convox`
 
### Grant Permissions
```html
    $ gcloud projects add-iam-policy-binding $GOOGLE_PROJECT --member=serviceAccount:${serviceaccount} --role=roles/owner
```
## Enable GCP APIs
```html
    $ gcloud services enable compute.googleapis.com
    $ gcloud services enable cloudresourcemanager.googleapis.com
```
## Install Rack
```html
    $ convox rack install gcp <name> [param1=value1]...
```
### Available Parameters

| Name          | Default         | Description                                                                              |
| ------------- | --------------- | ---------------------------------------------------------------------------------------- |
| **node_type**   | **n1-standard-1** | Node instance type                                                                       |
| **preemptible** | **true**          | Use [preemptible](https://cloud.google.com/compute/docs/instances/preemptible) instances |
| **region**      | **us-east1**      | GCP Region                                                                               |
| **syslog**      |                 | Forward logs to a syslog endpoint (e.g. **tcp+tls://example.org:1234**)                    |
