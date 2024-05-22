---
title: "cidr"
draft: false
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
To set the `cidr` parameter, use the following command:
```html
$ convox rack params set cidr=10.2.0.0/16 -r rackName
Setting parameters... OK
```
This command sets the CIDR range for the VPC to `10.2.0.0/16`.

## Additional Information
Choosing an appropriate CIDR range is crucial for the efficient management of your network. Ensure that the CIDR range does not overlap with any existing networks to avoid IP address conflicts. The CIDR range you choose should accommodate the number of subnets and hosts you plan to deploy within your VPC. For more information on CIDR notation and VPC planning, refer to the [AWS VPC documentation](https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html#VPC_Sizing).
