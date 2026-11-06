---
title: "eks_api_server_private_access_cidrs"
description: "The eks_api_server_private_access_cidrs AWS rack parameter lists CIDR blocks allowed to reach the EKS API via the private endpoint on TCP 443."
slug: eks_api_server_private_access_cidrs
url: /configuration/rack-parameters/aws/eks_api_server_private_access_cidrs
---

# eks_api_server_private_access_cidrs

## Description
Comma-separated list of CIDR blocks allowed to access the EKS Kubernetes API via the cluster's private endpoint. Each CIDR becomes a TCP 443 ingress rule on the Rack's `<rack>-cluster` security group, which EKS attaches to the cluster's network interfaces.

When the cluster's private API endpoint is on, clients inside the VPC, or connected to it, reach the EKS API through the network interfaces EKS places in the Rack's subnets; with the public endpoint off, that is the only way in. Endpoint access is set separately, see [Cluster Endpoint Access](/installation/production-rack/aws#cluster-endpoint-access). By default, the `<rack>-cluster` security group has no ingress rules, so only pods and nodes inside the VPC can reach the API. This parameter lets VPN-connected users, peered VPCs, or on-premises networks reach the private API endpoint by whitelisting their source CIDRs.

## Default Value
The default value is an empty string (`""`), which creates no rules. This parameter does not change the cluster security group EKS creates, which carries node-to-control-plane traffic.

## Use Cases
- **VPN access to private clusters**: When your team connects to the VPC via AWS Client VPN, Site-to-Site VPN, or a third-party VPN, add the VPN client CIDR so `kubectl` and `convox` CLI commands work through the private endpoint.
- **Peered VPC access**: When applications or CI/CD runners in a peered VPC need to call the Kubernetes API, add the peer VPC's CIDR.
- **On-premises access via Direct Connect**: For hybrid environments using AWS Direct Connect, add the on-premises network CIDR.

## Setting Parameters
Set one or more CIDRs (comma-separated, no spaces):
```bash
$ convox rack params set eks_api_server_private_access_cidrs=10.0.0.0/8 -r rackName
Updating parameters... OK
```

Multiple CIDRs:
```bash
$ convox rack params set eks_api_server_private_access_cidrs=10.0.0.0/8,172.16.0.0/12,192.168.1.0/24 -r rackName
Updating parameters... OK
```

To remove all private access rules (revert to default):
```bash
$ convox rack params set eks_api_server_private_access_cidrs= -r rackName
Updating parameters... OK
```

## Additional Information
- Each CIDR creates a separate `aws_security_group_rule` on the `<rack>-cluster` security group. CIDRs are deduplicated, so passing the same CIDR twice has no effect.
- Reordering CIDRs in the parameter value does not cause Terraform to destroy and recreate rules (the implementation uses `for_each` with set semantics, not index-based `count`).
- Invalid CIDR notation (e.g., missing prefix length) is rejected by the AWS API at apply time with a clear error message.
- This parameter only adds ingress rules to the `<rack>-cluster` security group; it does not change which EKS API endpoints are exposed. Public and private endpoint visibility is configured separately, through the Console on Console-managed racks. Adding the relevant CIDRs here is a prerequisite for reaching the private endpoint from outside the VPC.
- The rules stay with the interfaces when [eks_control_plane_private_subnets](/configuration/rack-parameters/aws/eks_control_plane_private_subnets) moves them, so they apply in either placement. Clients outside the VPC also need a return route on the route tables of the subnets the interfaces are in, see [Preparing Routes](/configuration/rack-parameters/aws/eks_control_plane_private_subnets#preparing-routes).
- Downgrade safety: removing this parameter (or downgrading to a rack version that does not support it) cleanly removes the security group rules. No orphaned resources.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
