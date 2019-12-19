# Convox on AWS

## Initial Setup

- Create an IAM user with the `AdministratorAccess` policy
- Create Access Credentials for this IAM user 
- Note these credentials

## Configuration

### Environment Variables

- `AWS_DEFAULT_REGION` (required)
- `AWS_ACCESS_KEY_ID` (required)
- `AWS_SECRET_ACCESS_KEY` (required)

## Install Convox

- Clone this repository and switch to the directory containing this `README`
- Run `terraform init`
- Run `terraform apply`

## Convox CLI Setup

- [Install the Convox CLI](../../docs/guides/installation#cli)
- Run `export RACK_URL=$(terraform output rack_url)`
- Run `convox rack` to ensure that your CLI is connected to your new Rack