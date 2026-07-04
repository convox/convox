---
title: "pod_imds_block_enabled"
description: "The pod_imds_block_enabled AWS rack parameter blocks App pod egress to the EC2 instance metadata service (IMDS), defaulting to false."
slug: pod_imds_block_enabled
url: /configuration/rack-parameters/aws/pod_imds_block_enabled
---

# pod_imds_block_enabled

## Description

The `pod_imds_block_enabled` parameter blocks App pod egress to the EC2 instance metadata service (IMDS) at `169.254.169.254`. When enabled, the Rack turns on the AWS VPC CNI NetworkPolicy controller and maintains a `convox-imds-block` NetworkPolicy in every App namespace. The policy is egress only: it allows all egress to `0.0.0.0/0` except `169.254.169.254/32`, so DNS, in-cluster traffic, and internet egress continue to work unchanged. Ingress is unaffected.

Blocking pod access to IMDS prevents a compromised workload from reading the node instance role credentials, a hardening step recommended by the AWS EKS Best Practices Guide and the CIS Amazon EKS Benchmark. Workloads then authenticate to AWS through their own identities instead of the node role. IRSA does not use IMDS, and the EKS Pod Identity endpoint (`169.254.170.23`) remains reachable, so both continue to work with the block in place.

The Rack reconciles the policy every 2 minutes, so namespaces for newly created Apps are covered automatically. The block applies to Service and resource pods in App namespaces. One-off `convox build` and `convox run` pods are standalone pods, which the AWS VPC CNI does not enforce NetworkPolicy on, so they are not covered. Timer pods are Job-owned rather than Deployment-backed, and VPC CNI enforcement on Job-owned pods has not been verified, so treat Timers as not covered as well. System and Rack namespaces are exempt and retain IMDS access.

This parameter is available on AWS Racks only and requires Rack version `3.24.10` or later.

## Default Value

The default value is `false`. With the default, the VPC CNI addon configuration is unchanged and no NetworkPolicy is created, so existing Racks see no change in behavior after upgrading.

## Setting the Parameter

```bash
$ convox rack params set pod_imds_block_enabled=true -r rackName
Updating parameters... OK
```

Applying the change updates the VPC CNI addon configuration, which rolls the `aws-node` DaemonSet across the cluster and can take several minutes. App pods are not restarted.

Before enabling, review the following:

- Workloads that rely on the AWS SDK falling back to IMDS for credentials (no IRSA or EKS Pod Identity configured) lose AWS API access. Move them to IRSA or EKS Pod Identity first.
- Workloads that read the region from `169.254.169.254` should switch to the `AWS_REGION` environment variable. The availability zone and instance ID are not available as pod environment fields; workloads that need them should receive the values as environment variables set at deploy time, or read them from the node object through the Kubernetes API (the zone is a node label and the instance ID is part of the node provider ID).
- Enabling this parameter activates the VPC CNI NetworkPolicy controller cluster-wide, so any NetworkPolicy objects previously applied by hand (and previously not enforced) become live at the same time. Review hand-applied policies before enabling.

Setting the parameter back to `false` removes the policy from all App namespaces and reverts the CNI addon configuration, restoring prior behavior with no node replacement and no manual cleanup.

## Additional Information

- **Validation:** Must be `true` or `false`.
- The node-level IMDS token settings, including [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens), are not changed by this parameter.

## See Also

- [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens)
- [pod_identity_agent_enable](/configuration/rack-parameters/aws/pod_identity_agent_enable)
