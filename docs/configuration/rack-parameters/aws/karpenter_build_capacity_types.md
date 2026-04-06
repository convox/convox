---
title: "karpenter_build_capacity_types"
slug: karpenter_build_capacity_types
url: /configuration/rack-parameters/aws/karpenter_build_capacity_types
---

# karpenter_build_capacity_types

## Description

The `karpenter_build_capacity_types` parameter sets the EC2 purchasing model for [Karpenter](/configuration/scaling/karpenter) build nodes.

## Default Value

The default value is `on-demand`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_build_capacity_types=on-demand,spot -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be `on-demand`, `spot`, or `on-demand,spot`.
- The build NodePool is only created when [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_capacity_types](/configuration/rack-parameters/aws/karpenter_capacity_types)
