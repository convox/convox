---
title: "private_subnets_ids"
description: "The private_subnets_ids AWS rack parameter sets the private subnet IDs used to create the Rack, defaulting to empty so Convox creates its own."
slug: private_subnets_ids
url: /configuration/rack-parameters/aws/private_subnets_ids
---

# private_subnets_ids

## Description
The `private_subnets_ids` parameter specifies the IDs of private subnets to use for creating the Rack. This is an advanced configuration parameter intended for edge use cases where the cluster needs to be installed into existing subnets.

## Default Value
The default value for `private_subnets_ids` is an empty string. When set to an empty string, Convox will automatically create private subnets within the VPC.

## Use Cases
- **Existing VPC Integration**: Use this parameter to integrate your Convox rack with existing private subnets in a VPC.
- **Custom Network Configuration**: Specify custom subnet IDs to meet specific network requirements and configurations.

## Setting Parameters
The `private_subnets_ids` parameter must be configured at rack installation. Example:

| Key                    | Value                                         |
|------------------------|-----------------------------------------------|
| `private_subnets_ids`  | `subnet-12345678,subnet-87654321,subnet-11223344` |

## Additional Information
When configuring `private_subnets_ids`, ensure that you also set the [vpc_id](/configuration/rack-parameters/aws/vpc_id) parameter and properly configure the VPC with a NAT gateway and route table. Additionally, configure the [public_subnets_ids](/configuration/rack-parameters/aws/public_subnets_ids) parameter, as the load balancer will use public subnets. For high availability, there should be at least three subnets.

Using this parameter lets you place rack workloads into private subnets you already manage rather than having Convox create new ones. This advanced configuration is suitable when your internal services must run inside an existing private network layout.
