---
title: "eks_api_server_private_access_cidrs"
description: "The eks_api_server_private_access_cidrs AWS rack parameter lists CIDR blocks allowed to reach the EKS API via the private endpoint on TCP 443."
slug: eks_api_server_private_access_cidrs
url: /configuration/rack-parameters/aws/eks_api_server_private_access_cidrs
---

# eks_api_server_private_access_cidrs

## Description
Comma-separated list of CIDR blocks allowed to access the EKS Kubernetes API via the cluster's private endpoint. Each CIDR becomes an ingress rule on the cluster security group allowing TCP 443 inbound.

When `disable_public_access=true` (or `enable_private_access=true` without public access), the EKS API is reachable only through the VPC's private endpoint ENIs. By default, the cluster security group has no additional ingress rules, so only pods and nodes inside the VPC can reach the API. This parameter lets VPN-connected users, peered VPCs, or on-premises networks reach the private API endpoint by whitelisting their source CIDRs.

## Default Value
The default value is an empty string (`""`), which means no additional security group rules are created. The cluster security group retains only the rules EKS creates by default (node-to-control-plane communication).

## Use Cases
- **VPN access to private clusters**: When your team connects to the VPC via AWS Client VPN, Site-to-Site VPN, or a third-party VPN, add the VPN client CIDR so `kubectl` and `convox` CLI commands work through the private endpoint.
- **Peered VPC access**: When applications or CI/CD runners in a peered VPC need to call the Kubernetes API, add the peer VPC's CIDR.
- **On-premises access via Direct Connect**: For hybrid environments using AWS Direct Connect, add the on-premises network CIDR.

## Setting Parameters
Set one or more CIDRs (comma-separated, no spaces):
```bash
$ convox rack params set eks_api_server_private_access_cidrs=10.0.0.0/8 -r rackName
Setting parameters... OK
```

Multiple CIDRs:
```bash
$ convox rack params set eks_api_server_private_access_cidrs=10.0.0.0/8,172.16.0.0/12,192.168.1.0/24 -r rackName
Setting parameters... OK
```

To remove all private access rules (revert to default):
```bash
$ convox rack params set eks_api_server_private_access_cidrs= -r rackName
Setting parameters... OK
```

## Additional Information
- Each CIDR creates a separate `aws_security_group_rule` on the cluster security group. CIDRs are deduplicated, so passing the same CIDR twice has no effect.
- Reordering CIDRs in the parameter value does not cause Terraform to destroy and recreate rules (the implementation uses `for_each` with set semantics, not index-based `count`).
- Invalid CIDR notation (e.g., missing prefix length) is rejected by the AWS API at apply time with a clear error message.
- This parameter only adds ingress rules to the cluster security group; it does not change which EKS API endpoints are exposed. Public and private endpoint visibility is configured separately, through the Console on Console-managed racks. Adding the relevant CIDRs here is a prerequisite for reaching the private endpoint from outside the VPC.
- Downgrade safety: removing this parameter (or downgrading to a rack version that does not support it) cleanly removes the security group rules. No orphaned resources.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
