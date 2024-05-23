---
title: "vpc_id"
draft: false
slug: vpc_id
url: /configuration/rack-parameters/aws/vpc_id
---

# vpc_id

## Description
The `vpc_id` parameter specifies the ID of an existing VPC to use for cluster creation. When using this parameter, ensure that you also configure the [cidr](/configuration/rack-parameters/aws/cidr) block and [internet_gateway_id](/configuration/rack-parameters/aws/internet_gateway_id).

## Default Value
The default value for `vpc_id` is ``. When set to ``, Convox will create a new VPC for the cluster.

## Use Cases
- **Existing VPC Integration**: Use this parameter to integrate your Convox rack with an existing VPC.
- **Custom Network Configuration**: Specify an existing VPC to meet specific network requirements and configurations.

## Setting Parameters
The `vpc_id` parameter must be configured at rack installation. Example:
| Key                    | Value                                         |
|------------------------|-----------------------------------------------|
| `vpc_id`   | `vpc-12345678` |

&nbsp;

## Additional Information
When configuring the `vpc_id` parameter, ensure that you also set the [cidr](/configuration/rack-parameters/aws/cidr) block and [internet_gateway_id](/configuration/rack-parameters/aws/internet_gateway_id) parameters. Additionally, configure the [private_subnets_ids](/configuration/rack-parameters/aws/private_subnets_ids) and [public_subnets_ids](/configuration/rack-parameters/aws/public_subnets_ids) parameters for subnet configurations. Proper configuration of these parameters is essential for integrating your Convox rack with an existing VPC and ensuring network connectivity and security.

By setting the `vpc_id` parameter, you can leverage existing network infrastructure and customize the VPC configuration for your Convox rack.
