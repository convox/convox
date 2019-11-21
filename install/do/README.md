# Convox Rack on Digital Ocean

## Initial Setup

- Log in to your Digital Ocean Dashboard
- Go to https://cloud.digitalocean.com/account/api/tokens
- Generate a new **Personal Access Token** and **Spaces Access Key**
- Note these credentials

## Install Convox

- Clone this repository and switch to the directory containing this `README`
- Run `terraform init`
- Run `terraform apply`

## Convox CLI Setup

- [Install the Convox CLI](https://docs.convox.com/introduction/installation)
- Run `export RACK_URL=$(terraform output rack_url)`
- Run `convox rack` to ensure that your CLI is connected to your new Rack
