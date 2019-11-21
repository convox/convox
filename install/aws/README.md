# Convox Rack on AWS

## Initial Setup

- [Install the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)
- [Configure the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html)

## Install Convox

- Clone this repository and switch to the directory containing this `README`
- Run `terraform init`
- Run `terraform apply`

## Convox CLI Setup

- [Install the Convox CLI](https://docs.convox.com/introduction/installation)
- Run `export RACK_URL=$(terraform output rack_url)`
- Run `convox rack` to ensure that your CLI is connected to your new Rack