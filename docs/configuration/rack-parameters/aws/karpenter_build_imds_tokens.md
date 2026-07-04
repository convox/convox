---
title: "karpenter_build_imds_tokens"
description: "The karpenter_build_imds_tokens AWS rack parameter sets the IMDS session token requirement (IMDSv2) on Karpenter build nodes independently of the rack-wide setting."
slug: karpenter_build_imds_tokens
url: /configuration/rack-parameters/aws/karpenter_build_imds_tokens
---

# karpenter_build_imds_tokens

## Description

The `karpenter_build_imds_tokens` parameter sets the Instance Metadata Service (IMDS) session token requirement on [Karpenter](/configuration/scaling/karpenter) build nodes independently of the rack-wide [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens) setting.

- `optional`: Build nodes allow metadata access with or without a session token.
- `required`: Build nodes require IMDSv2 session tokens for all metadata access.
- Empty (default): Build nodes inherit the rack-wide `imds_http_tokens` value.

When set, the value is rendered into the `metadataOptions.httpTokens` field of the build `EC2NodeClass`. Build nodes run user-supplied Dockerfile instructions during Builds, so they often warrant stricter instance metadata settings than the nodes running application workloads. This parameter applies only to the Karpenter build NodePool and takes effect only when [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled) and [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) are both `true`. The workload NodePool, additional Karpenter NodePools, the system node group, and the rack-wide `imds_http_tokens` parameter are unaffected.

This parameter is available on AWS Racks only and requires Rack version `3.24.10` or later.

## Default Value

The default value is `""` (empty), which inherits the rack-wide `imds_http_tokens` setting. A Rack that never sets this parameter renders an identical build node configuration.

## Setting the Parameter

```bash
$ convox rack params set karpenter_build_imds_tokens=required -r rackName
Updating parameters... OK
```

Changing this value re-renders the build `EC2NodeClass`. Karpenter detects the drift and replaces build nodes with instances using the new metadata options, and new Builds land on the reconfigured nodes.

To return to the inherited rack-wide setting, clear the parameter:

```bash
$ convox rack params set karpenter_build_imds_tokens="" -r rackName
Updating parameters... OK
```

## Additional Information

- **Validation:** Must be `optional`, `required`, or empty. Values are case sensitive.
- **Before requiring tokens**, confirm that your Build steps do not read instance metadata without IMDSv2 session tokens (for example, through an older AWS SDK or a plain HTTP request to the metadata endpoint).
- The parameter appears in both the `security` and `build` groups of `convox rack params -g <group>`.
- Pair with [karpenter_build_imds_hop_limit](/configuration/rack-parameters/aws/karpenter_build_imds_hop_limit) to also limit the metadata response hop count on build nodes.

## See Also

- [karpenter_build_imds_hop_limit](/configuration/rack-parameters/aws/karpenter_build_imds_hop_limit)
- [imds_http_hop_limit](/configuration/rack-parameters/aws/imds_http_hop_limit)
- [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens)
- [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled)
- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
