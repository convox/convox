# Google Cloud

## Initial Setup

### Google Cloud CLI

- [Install the Google Cloud CLI](https://cloud.google.com/sdk/docs/#install_the_latest_cloud_tools_version_cloudsdk_current_version)
- Run `gcloud init`

### Convox CLI

- [Install the Convox CLI](../cli.md)

## Environment

The following environment variables are required:

- `GOOGLE_CREDENTIALS`
- `GOOGLE_PROJECT`
- `GOOGLE_REGION`

### Select Region

    $ gcloud compute regions list

- `GOOGLE_REGION` is a region `NAME` from the list

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

    $ convox rack install gcp <name>
