---
title: "cidr"
description: "The cidr AWS rack parameter sets the CIDR range for the VPC, defining its IP address space, and defaults to 10.1.0.0/16."
slug: cidr
url: /configuration/rack-parameters/aws/cidr
---

# cidr

## Description
The `cidr` parameter specifies the Classless Inter-Domain Routing (CIDR) range for the Virtual Private Cloud (VPC). This range defines the IP address space for your VPC, allowing you to organize and allocate IP addresses within your AWS environment.

## Default Value
The default value for `cidr` is `10.1.0.0/16`.

## Use Cases
- **Custom IP Range**: Define a specific IP address range to avoid conflicts with existing networks or to meet organizational policies.
- **Network Segmentation**: Organize your IP addresses to support subnetting and network segmentation within your VPC.

## Setting Parameters
The `cidr` parameter must be configured at rack installation. Example:

| Key                    | Value                                         |
|------------------------|-----------------------------------------------|
| `cidr`  | `10.2.0.0/16` |

## Additional Information
Choosing an appropriate CIDR range is crucial for the efficient management of your network. Ensure that the CIDR range does not overlap with any existing networks to avoid IP address conflicts. The CIDR range you choose should accommodate the number of subnets and hosts you plan to deploy within your VPC. For more information on CIDR notation and VPC planning, refer to the [AWS VPC documentation](https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html#VPC_Sizing).

## Subnet Layout
Convox divides the `cidr` range into these subnets:

| Subnets | Prefix length | With the default `10.1.0.0/16` |
|---------|---------------|--------------------------------|
| public 0, 1, 2 | `cidr` prefix length plus 4 | `10.1.0.0/20`, `10.1.16.0/20`, `10.1.32.0/20` |
| private 0, 1, 2 | `cidr` prefix length plus 2 | `10.1.64.0/18`, `10.1.128.0/18`, `10.1.192.0/18` |

The public subnets are the first three sixteenths of the range and the private subnets the second, third and fourth quarters. The third subnet of each kind exists only with [high_availability](/configuration/rack-parameters/aws/high_availability) set to `true`, and private subnets exist only with [private](/configuration/rack-parameters/aws/private) set to `true`. A Rack installed into existing subnets with [private_subnets_ids](/configuration/rack-parameters/aws/private_subnets_ids) and [public_subnets_ids](/configuration/rack-parameters/aws/public_subnets_ids) uses those subnets instead.
