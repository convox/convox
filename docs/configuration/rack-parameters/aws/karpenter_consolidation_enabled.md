---
title: "karpenter_consolidation_enabled"
slug: karpenter_consolidation_enabled
url: /configuration/rack-parameters/aws/karpenter_consolidation_enabled
---

# karpenter_consolidation_enabled

## Description

The `karpenter_consolidation_enabled` parameter controls how aggressively [Karpenter](/configuration/scaling/karpenter) consolidates workload nodes.

## Default Value

The default value is `true`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_consolidation_enabled=false -r rackName
Setting parameters... OK
```

## Additional Information

- When `true`: Karpenter uses `WhenEmptyOrUnderutilized` — it consolidates both underutilized nodes (by moving pods to fewer, better-utilized nodes) and fully empty nodes.
- When `false`: Karpenter uses `WhenEmpty` — it only removes nodes with no running pods.
- The delay before consolidation triggers is controlled by [`karpenter_consolidate_after`](/configuration/rack-parameters/aws/karpenter_consolidate_after).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_consolidate_after](/configuration/rack-parameters/aws/karpenter_consolidate_after)
- [karpenter_disruption_budget_nodes](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes)
