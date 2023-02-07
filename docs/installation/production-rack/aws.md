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

| Name                     | Default                | Description                                                                                                    |
| -------------------------|------------------------|----------------------------------------------------------------------------------------------------------------|
| **availability_zones**   |                        | Specify a list of AZ names (minimum 3) to override the random automatic selection from AWS                     |
| **cert_duration**        | **2160h**              | Certification renew period                                                                                     | 
| **cidr**                 | **10.1.0.0/16**        | CIDR range for VPC                                                                                             |
| **high_availability**    | **true**               | Setting this to "false" will create a cluster with less reduntant resources for cost optimization              |
| **internet_gateway_id**  |                        | If you're using an existing vpc for your rack, use this field to pass the id of the attached internet gateway  |
| **idle_timeout**         | **3600**               | Idle timeout value (in seconds) for the Rack Load Balancer                                                     |
| **key_pair_name**        |                        | AWS key pair to use for ssh|
| **node_capacity_type**   | **on_demand**          | Can be either "on_demand" or "spot". Spot will use AWS spot instances for the cluster nodes                    |
| **node_disk**            | **20**                 | Node disk size in GB                                                                                           |
| **node_type**            | **t3.small**           | Node instance type. You can also pass a comma separated list of instance types                                 |
| **schedule_rack_scale_down**   |                        | Rack scale down schedule is specified by the user following the Unix cron syntax format. Example: "0 18 * * 5". The supported cron expression format consists of five fields separated by white spaces: [Minute] [Hour] [Day_of_Month] [Month_of_Year] [Day_of_Week]. More details on the CRON format can be found in (Crontab)[http://crontab.org/] and (examples)[https://crontab.guru/examples.html]. The time is calculated in **UTC**. |
| **schedule_rack_scale_up**    |                        | Rack scale up schedule is specified by the user following the Unix cron syntax format.Example: "0 0 * * 0". The supported cron expression format consists of five fields separated by white spaces: [Minute] [Hour] [Day_of_Month] [Month_of_Year] [Day_of_Week]. More details on the CRON format can be found in (Crontab)[http://crontab.org/] and (examples)[https://crontab.guru/examples.html]. The time is calculated in **UTC**. |
| **private**              | **false**               | Put nodes in private subnets behind NAT gateways                                                               |
| **proxy_protocol**       | **false**               | Enable proxy protocol. With this parameter set, the client source ip will be available in the request header `x-forwarded-for` key. **Requires 5 - 10 minutes downtime**.          |
| **region**               | **us-east-1**          | AWS Region                                                                                                     |
| **syslog**               |                        | Forward logs to a syslog endpoint (e.g. **tcp+tls://example.org:1234**)                                        |
| **tags**                 |                        | Custom tags to add with AWS resources (e.g. **key1=val1,key2=val2**)|
| **vpc_id** *             |                        | Use an existing VPC for cluster creation. Make sure to also pass the **cidr** block and **internet_gateway_id**|

\* To avoid CIDR block collision with existing VPC subnets, please add a new CIDR block to your VPC to separate rack resources. Also, remember to pass the **internet_gateway_id** attached to the VPC. If the VPC doesn't have an IG attached, the rack installation will create one automatically, which will also be destroyed if you delete the rack.

\* **schedule_rack_scale_down** and **schedule_rack_scale_up** are mutually exclusive. So you have to set both of them properly for the scheduled timed off. If you set only **schedule_rack_scale_down**, it will not scale up on its own.
