---
title: "public_subnets_ids"
draft: false
slug: public_subnets_ids
url: /configuration/rack-parameters/aws/public_subnets_ids
---

# public_subnets_ids

## Description
The `public_subnets_ids` parameter specifies the IDs of public subnets to use for creating the Rack. This is an advanced configuration parameter intended for edge use cases where the cluster needs to be installed into existing subnets.

## Default Value
The default value for `public_subnets_ids` is an empty string. When set to an empty string, Convox will automatically create public subnets within the VPC.

## Use Cases
- **Existing VPC Integration**: Use this parameter to integrate your Convox rack with existing public subnets in a VPC.
- **Custom Network Configuration**: Specify custom subnet IDs to meet specific network requirements and configurations.

## Setting Parameters
The `public_subnets_ids` parameter must be configured at rack installation. Example:
| Key                    | Value                                         |
|------------------------|-----------------------------------------------|
| `public_subnets_ids`   | `subnet-12345678,subnet-87654321,subnet-11223344` |

&nbsp;

## Additional Information
When configuring `public_subnets_ids`, ensure that you also set the [vpc_id](/configuration/rack-parameters/aws/vpc_id) parameter and properly configure the VPC with an internet gateway and route table. Additionally, configure the [private_subnets_ids](/configuration/rack-parameters/aws/private_subnets_ids) parameter for internal resources. For high availability, there should be at least three subnets.

Using this parameter allows you to leverage existing network infrastructure and customize the subnet configuration for your Convox rack. This advanced configuration is suitable for scenarios where specific network setups are required.
