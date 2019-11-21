# Convox Rack on GCP

## Initial Setup

- Go to your GCP Dashboard
- Create a new project
- Note the ID of the project
- Go to https://console.cloud.google.com/apis/credentials/serviceaccountkey
- Ensure you have your new project selected from the dropdown
- Select **New Service Account**
- Give it a name like `terraform`
- Give it the **Project Owner** role
- Select key type **JSON**
- Click **Create**
- Download the credential file to `~/.config/gcloud/terraform.json`

## Install Convox

- Clone this repository and switch to the directory containing this `README`
- Run `terraform init`
- Run `terraform apply -target module.system.module.project` to enable necessary services in your project
- Run `terraform apply`

## Convox CLI Setup

- [Install the Convox CLI](https://docs.convox.com/introduction/installation)
- Run `export RACK_URL=$(terraform output rack_url)`
- Run `convox rack` to ensure that your CLI is connected to your new Rack
