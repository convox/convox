---
title: "karpenter_build_imds_hop_limit"
description: "The karpenter_build_imds_hop_limit AWS rack parameter sets the IMDS response hop limit on Karpenter build nodes independently of the rack-wide setting."
slug: karpenter_build_imds_hop_limit
url: /configuration/rack-parameters/aws/karpenter_build_imds_hop_limit
---

# karpenter_build_imds_hop_limit

## Description

The `karpenter_build_imds_hop_limit` parameter sets the EC2 Instance Metadata Service (IMDS) PUT response hop limit (`httpPutResponseHopLimit`) on [Karpenter](/configuration/scaling/karpenter) build nodes, independently of the rack-wide [imds_http_hop_limit](/configuration/rack-parameters/aws/imds_http_hop_limit) setting.

Build nodes run user-supplied Dockerfile instructions during `convox build` and `convox deploy`, so they often warrant stricter instance-metadata settings than workload nodes. A hop limit of `1` stops IMDSv2 token responses from crossing the extra network hop into containers, which keeps Build containers from acquiring node credentials through IMDSv2. Pair it with [karpenter_build_imds_tokens](/configuration/rack-parameters/aws/karpenter_build_imds_tokens)=`required` to also shut off tokenless IMDSv1 requests and restrict instance-metadata access to the node itself.

The value renders into the `metadataOptions` of the build `EC2NodeClass` only. The workload NodePool, additional Karpenter NodePools, the system node group, and the rack-wide IMDS parameters are unaffected.

This parameter applies only when [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled) and [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) are both `true`. Without dedicated build nodes there is no build NodePool to configure.

## Default Value

The default value is `0`, which inherits the rack-wide `imds_http_hop_limit` value (`3` by default). A Rack that never sets this parameter renders an identical build `EC2NodeClass` configuration before and after upgrading, so existing behavior is unchanged.

## Setting the Parameter

```bash
$ convox rack params set karpenter_build_imds_hop_limit=1 -r rackName
Updating parameters... OK
```

Changing the value re-renders the build `EC2NodeClass`. Karpenter detects the drift and replaces build nodes with instances carrying the new metadata options. Karpenter's drift replacement does not voluntarily disrupt nodes running active Build pods, so in-flight Builds run to completion on the existing nodes and new Builds land on the reconfigured ones.

To return to the inherited rack-wide setting, set the value back to `0`:

```bash
$ convox rack params set karpenter_build_imds_hop_limit=0 -r rackName
Updating parameters... OK
```

This restores the inherited options with no manual cleanup.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.24.10` or later.

- **Validation:** must be a non-negative integer. The CLI rejects negative values, non-integer values, and an empty value. Revert with `0`, not an empty string.
- **Before setting `1`:** confirm your Build steps do not read instance metadata from inside the Build container (for example, an SDK resolving AWS credentials from the node role during `docker build`). Build steps that need AWS credentials should receive them through Build arguments or environment configuration rather than the node's instance metadata.
- **Parameter groups:** listed under both the `build` and `security` groups (`convox rack params -g security -r rackName`).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_build_imds_tokens](/configuration/rack-parameters/aws/karpenter_build_imds_tokens)
- [imds_http_hop_limit](/configuration/rack-parameters/aws/imds_http_hop_limit)
- [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens)
- [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled)
