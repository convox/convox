---
title: "imds_http_hop_limit"
description: "The imds_http_hop_limit AWS rack parameter sets the EC2 Instance Metadata Service (IMDS) response hop limit on all Rack nodes, defaulting to 3."
slug: imds_http_hop_limit
url: /configuration/rack-parameters/aws/imds_http_hop_limit
---

# imds_http_hop_limit

## Description
The `imds_http_hop_limit` parameter sets the EC2 Instance Metadata Service (IMDS) PUT response hop limit (`httpPutResponseHopLimit`) on the EC2 instances the Rack launches. It applies to the launch templates for the system node group, the build node group, and any additional node groups, and to the `metadataOptions` of the Karpenter `EC2NodeClass` resources for workload, build, and additional NodePools.

The hop limit controls how many network hops an IMDSv2 token response can travel from the instance. A value of `1` keeps token responses on the node itself, so processes inside containers (which sit behind an extra network hop) cannot complete IMDSv2 token acquisition. Values of `2` or higher allow containerized processes, including pods, to obtain IMDSv2 tokens and read instance metadata.

The hop limit affects IMDSv2 token responses only. With [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens) at its default of `optional`, tokenless IMDSv1 requests still succeed from containers regardless of the hop limit. To restrict instance metadata access to node-level processes, set `imds_http_hop_limit=1` together with `imds_http_tokens=required`.

## Default Value
The default value for `imds_http_hop_limit` is `3`. This allows containerized workloads to reach IMDS through the extra hops that container networking introduces, preserving compatibility with applications that read instance metadata, such as AWS SDKs resolving credentials from the node role.

## Use Cases
- **Hardening**: Setting the hop limit to `1` (combined with `imds_http_tokens=required`) prevents App containers from reading node role credentials through IMDS, reducing the impact of a compromised workload.
- **Compatibility**: Keeping the default allows workloads without IRSA or EKS Pod Identity to keep using the node role credentials exposed through IMDS.

## Setting Parameters
To set the `imds_http_hop_limit` parameter, use the following command:
```bash
$ convox rack params set imds_http_hop_limit=1 -r rackName
Updating parameters... OK
```
Applying the change creates a new launch template version and triggers a rolling replacement of node group instances. When Karpenter is enabled, the updated `EC2NodeClass` resources cause Karpenter to replace nodes through drift, so new nodes carry the new metadata options.

## Additional Information
This parameter is available on AWS Racks since Rack version `3.13.8`.

Before lowering the value to `1` with `imds_http_tokens=required`, verify that workloads do not depend on IMDS. Pods that rely on the AWS SDK falling back to the node role for credentials, or that read the region or instance identity from IMDS, lose that access. Move them to IRSA or EKS Pod Identity first. To block pod access to IMDS without changing node-level metadata options, see [pod_imds_block_enabled](/configuration/rack-parameters/aws/pod_imds_block_enabled).

- **Value range**: EC2 accepts hop limit values from `1` to `64`.
- **Build node override**: [karpenter_build_imds_hop_limit](/configuration/rack-parameters/aws/karpenter_build_imds_hop_limit) sets a separate hop limit for Karpenter build nodes. At its default of `0`, build nodes inherit the `imds_http_hop_limit` value.
- **Karpenter config override**: a `metadataOptions` block in [karpenter_config](/configuration/rack-parameters/aws/karpenter_config) replaces the default metadata options on the workload `EC2NodeClass`, including this hop limit.
- **Non-HA Racks**: on a Rack with `high_availability=false`, the CLI rejects setting `imds_http_hop_limit` in the same call as `karpenter_enabled=true`. Apply the hop limit change first, wait for the update to complete, then enable Karpenter.
- **Parameter group**: listed under the `security` group (`convox rack params -g security -r rackName`).

## See Also

- [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens)
- [karpenter_build_imds_hop_limit](/configuration/rack-parameters/aws/karpenter_build_imds_hop_limit)
- [karpenter_build_imds_tokens](/configuration/rack-parameters/aws/karpenter_build_imds_tokens)
- [pod_imds_block_enabled](/configuration/rack-parameters/aws/pod_imds_block_enabled)
