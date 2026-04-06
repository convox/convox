---
title: "karpenter_build_consolidate_after"
slug: karpenter_build_consolidate_after
url: /configuration/rack-parameters/aws/karpenter_build_consolidate_after
---

# karpenter_build_consolidate_after

## Description

The `karpenter_build_consolidate_after` parameter sets the delay before empty [Karpenter](/configuration/scaling/karpenter) build nodes are consolidated (removed).

## Default Value

The default value is `60s`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_build_consolidate_after=120s -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be a duration like `60s`, `5m`, or `1h` (regex: `^\d+[smh]$`).
- After the last build completes, build nodes are removed after this delay. Lower values reclaim build node costs faster; higher values keep warm capacity for back-to-back builds.
- The build NodePool is only created when [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_consolidate_after](/configuration/rack-parameters/aws/karpenter_consolidate_after) for workload node consolidation
