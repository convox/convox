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
| **build_node_enabled**   |     false              | Enabled dedicated build node for build |
| **build_node_type**      | same as **node_type**  | Node type for the build node |
| **build_node_min_count** |     0                  | Minimum number of build nodes to keep running |
| **cert_duration**        | **2160h**              | Certification renew period                                                                                     | 
| **cidr**                 | **10.1.0.0/16**        | CIDR range for VPC                                                                                             |
| **gpu_tag_enable**       | **false**              | Enable gpu tagging. Some aws region doesn't support gpu tagging  |
| **high_availability**    | **true**               | Setting this to "false" will create a cluster with less reduntant resources for cost optimization              |
| **internal_router**  |     **false**        | Install an internal loadbalancer within the vpc |
| **internet_gateway_id**  |                        | If you're using an existing vpc for your rack, use this field to pass the id of the attached internet gateway  |
| **idle_timeout**         | **3600**               | Idle timeout value (in seconds) for the Rack Load Balancer                                                     |
| **imds_http_tokens**         | **optional**               | Whether or not the metadata service requires session tokens, also referred to as Instance Metadata Service Version 2 (IMDSv2). Can be optional or required                                                     |
| **key_pair_name**        |                        | AWS key pair to use for ssh|
| **min_on_demand_count**  | **1**                  | When used with `mixed` node capacity type, can set the minimum required number of on demand nodes              |
| **max_on_demand_count**  | **100**                | When used with `mixed` node capacity type, can set the maximum required number of on demand nodes              |
| **node_capacity_type**   | **on_demand**          | Can be either "on_demand", "spot" or "mixed". Spot will use AWS spot instances for the cluster nodes.  Mixed will create one node group with on demand instances, and the other 2 with spot instances.  Use mixed with the min_on_demand_count and max_on_demand_count parameters to control the minimum acceptable service availability should all spot instances become unavailable.  |
| **node_disk**            | **20**                 | Node disk size in GB                                                                                           |
| **node_type**            | **t3.small**           | AWS instance type used for the cluster.                                                                        |
| **schedule_rack_scale_down**   |                        | Rack scale down schedule is specified by the user following the Unix cron syntax format. Example: "0 18 * * 5". The supported cron expression format consists of five fields separated by white spaces: [Minute] [Hour] [Day_of_Month] [Month_of_Year] [Day_of_Week]. More details on the CRON format can be found in (Crontab)[http://crontab.org/] and (examples)[https://crontab.guru/examples.html]. The time is calculated in **UTC**. |
| **schedule_rack_scale_up**    |                        | Rack scale up schedule is specified by the user following the Unix cron syntax format.Example: "0 0 * * 0". The supported cron expression format consists of five fields separated by white spaces: [Minute] [Hour] [Day_of_Month] [Month_of_Year] [Day_of_Week]. More details on the CRON format can be found in (Crontab)[http://crontab.org/] and (examples)[https://crontab.guru/examples.html]. The time is calculated in **UTC**. |
| **private**              | **true**               | Put nodes in private subnets behind NAT gateways                                                               |
| **proxy_protocol**       | **false**               | Enable proxy protocol. With this parameter set, the client source ip will be available in the request header `x-forwarded-for` key. **Requires 5 - 10 minutes downtime**. This is not applicable for **internal_router**        |
| **region**               | **us-east-1**          | AWS Region                                                                                                     |
| **syslog**               |                        | Forward logs to a syslog endpoint (e.g. **tcp+tls://example.org:1234**)                                        |
| **ssl_ciphers**          |                        | SSL ciphers to use for (nginx)[https://nginx.org/en/docs/http/ngx_http_ssl_module.html#ssl_ciphers]. They must be separated by colon.|
| **ssl_protocols**        |                        | SSL protocols to use for (nginx)[https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ssl_protocols] (e.g. **TLSv1.2 TLSv1.3**). They must be separated by spaces.|
| **tags**                 |                        | Custom tags to add with AWS resources (e.g. **key1=val1,key2=val2**)|
| **vpc_id** *             |                        | Use an existing VPC for cluster creation. Make sure to also pass the **cidr** block and **internet_gateway_id**|
| **private_subnets_ids**  |                        | Ids of private subnets to use to create the Rack. Make sure to also pass the **vpc_id** and it should be properly configured with nat gateway and route table. Also configure the public subnets since load balancer will use public subnets. For high availability there should be 3 subnets. Use comma to specify multiple subnets(no space)|
| **public_subnets_ids**   |                        | Ids of private subnets to use to create the Rack. Make sure to also pass the **vpc_id** and it should be properly configured with internet gateway and route table. For high availability there should be 3 subnets. Use comma to specify multiple subnets(no space)|

\* To avoid CIDR block collision with existing VPC subnets, please add a new CIDR block to your VPC to separate rack resources. Also, remember to pass the **internet_gateway_id** attached to the VPC. If the VPC doesn't have an IG attached, the rack installation will create one automatically, which will also be destroyed if you delete the rack.

\* **schedule_rack_scale_down** and **schedule_rack_scale_up** are mutually exclusive. So you have to set both of them properly for the scheduled timed off. If you set only **schedule_rack_scale_down**, it will not scale up on its own.
