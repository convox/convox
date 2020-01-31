# Amazon Web Services

## Initial Setup

### AWS CLI

- [Install the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)

### Convox CLI

- [Install the Convox CLI](../cli.md)

## Environment

The following environment variables are required:

- `AWS_ACCESS_KEY_ID`
- `AWS_DEFAULT_REGION`
- `AWS_SECRET_ACCESS_KEY`

### Select Region

You can list all available regions for your account with the following command:

    $ aws ec2 describe-regions --all-regions

- `AWS_DEFAULT_REGION` is `RegionName`

### Create IAM User

    $ aws iam create-user --user-name convox
    $ aws iam create-access-key --user-name convox

- `AWS_ACCESS_KEY_ID` is `AccessKeyId`
- `AWS_SECRET_ACCESS_KEY` is `SecretAccessKey`

### Grant Permissions

    $ aws iam attach-user-policy --user-name convox --policy-arn arn:aws:iam::aws:policy/AdministratorAccess

## Install Rack

    $ convox rack install aws <name>
