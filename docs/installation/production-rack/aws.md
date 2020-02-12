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
    $ aws iam attach-user-policy --user-name convox --policy-arn arn:aws:iam::aws:policy/AdministratorAccess
    $ aws iam create-access-key --user-name convox

- `AWS_ACCESS_KEY_ID` is `AccessKeyId`
- `AWS_SECRET_ACCESS_KEY` is `SecretAccessKey`

## Install Rack

    $ convox rack install aws <name> [param1=value1]...

### Available Parameters

| Name        | Default       |
| ----------- | ------------- |
| `cidr`      | `10.1.0.0/16` |
| `node_disk` | `20`          |
| `node_type` | `t3.small`    |
| `region`    | `us-east-1`   |