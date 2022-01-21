---
title: "Amazon Web Services"
draft: false
slug: Amazon Web Services
url: /installation/production-rack/aws
---
# Amazon Web Services
> Please note that these are instructions for installing a Rack via the command line. The easiest way to install a Rack is with the [Convox Web Console](https://console.convox.com)

## Initial Setup

### AWS CLI

- [Install the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)

### Terraform

- Install [Terraform](https://learn.hashicorp.com/terraform/getting-started/install.html)

### Convox CLI

- [Install the Convox CLI](/installation/cli)

## Environment

The following environment variables are required:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`

### Create IAM User
```html
    $ aws iam create-user --user-name convox
    $ aws iam attach-user-policy --user-name convox --policy-arn arn:aws:iam::aws:policy/AdministratorAccess
    $ aws iam create-access-key --user-name convox
```
- `AWS_ACCESS_KEY_ID` is `AccessKeyId`
- `AWS_SECRET_ACCESS_KEY` is `SecretAccessKey`

## Install Rack
```html
    $ convox rack install aws <name> [param1=value1]...
```
### Available Parameters

| Name                   | Default         | Description                                                                                |
| -----------------------| ----------------|--------------------------------------------------------------------------------------------|
| **availability_zones** |                 | Specify a list of AZ names (minimum 3) to override the random automatic selection from AWS |
| **cidr**               | **10.1.0.0/16** | CIDR range for VPC                                                                         |
| **idle_timeout**       | **3600**        | Idle timeout value (in seconds) for the Rack Load Balancer                                 |
| **node_disk**          | **20**          | Node disk size in GB                                                                       |
| **node_type**          | **t3.small**    | Node instance type. You can also pass a comma separated list of instance types             |
| **private**            | **true**        | Put nodes in private subnets behind NAT gateways                                           |
| **region**             | **us-east-1**   | AWS Region                                                                                 |
| **syslog**             |                 | Forward logs to a syslog endpoint (e.g. **tcp+tls://example.org:1234**)                    |
