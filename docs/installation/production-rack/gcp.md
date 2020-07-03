# Google Cloud
> Please note that these are instructions for installing a Rack via the command line. The easiest way to install a Rack is with the [Convox Web Console](https://console.convox.com)

## Initial Setup

### Google Cloud CLI

- [Install the Google Cloud CLI](https://cloud.google.com/sdk/docs/#install_the_latest_cloud_tools_version_cloudsdk_current_version)
- Run `gcloud init`

### Terraform

- Install [Terraform](https://learn.hashicorp.com/terraform/getting-started/install.html)

### Convox CLI

- [Install the Convox CLI](../cli.md)

## Environment

The following environment variables are required:

- `GOOGLE_CREDENTIALS`
- `GOOGLE_PROJECT`

### Create Project

    $ gcloud projects create <id> --set-as-default

- `GOOGLE_PROJECT` is the id you selected

> You will likely need to set up Billing on this new project at https://console.cloud.google.com/billing before proceeding

### Create Service Account

    $ serviceaccount=$(gcloud iam service-accounts create convox --format="value(email)")
    $ gcloud iam service-accounts keys create ~/gcloud.convox --iam-account=${serviceaccount}
    
- `GOOGLE_CREDENTIALS` is `~/gcloud.convox`
 
### Grant Permissions

    $ gcloud projects add-iam-policy-binding $GOOGLE_PROJECT --member=serviceAccount:${serviceaccount} --role=roles/owner

## Install Rack

    $ convox rack install gcp <name> [param1=value1]...

### Available Parameters

| Name          | Default         | Description                                                                              |
| ------------- | --------------- | ---------------------------------------------------------------------------------------- |
| `node_type`   | `n1-standard-1` | Node instance type                                                                       |
| `preemptible` | `true`          | Use [preemptible](https://cloud.google.com/compute/docs/instances/preemptible) instances |
| `region`      | `us-east1`      | GCP Region                                                                               |