---
title: "karpenter_capacity_types"
slug: karpenter_capacity_types
url: /configuration/rack-parameters/aws/karpenter_capacity_types
---

# karpenter_capacity_types

## Description

The `karpenter_capacity_types` parameter sets the EC2 purchasing model for [Karpenter](/configuration/scaling/karpenter) workload nodes.

## Default Value

The default value is `on-demand`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_capacity_types=on-demand,spot -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be `on-demand`, `spot`, or `on-demand,spot`.
- When both are specified, Karpenter optimizes for cost and automatically falls back to on-demand when spot capacity is unavailable.
- Build nodes use [`karpenter_build_capacity_types`](/configuration/rack-parameters/aws/karpenter_build_capacity_types) independently.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_build_capacity_types](/configuration/rack-parameters/aws/karpenter_build_capacity_types)
- [node_capacity_type](/configuration/rack-parameters/aws/node_capacity_type) for the primary node group purchasing model
