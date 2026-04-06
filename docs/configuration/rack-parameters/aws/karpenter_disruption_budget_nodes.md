---
title: "karpenter_disruption_budget_nodes"
slug: karpenter_disruption_budget_nodes
url: /configuration/rack-parameters/aws/karpenter_disruption_budget_nodes
---

# karpenter_disruption_budget_nodes

## Description

The `karpenter_disruption_budget_nodes` parameter sets the maximum number of [Karpenter](/configuration/scaling/karpenter) workload nodes that can be disrupted simultaneously during consolidation or node replacement.

## Default Value

The default value is `10%`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_disruption_budget_nodes=3 -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be a number or percentage (e.g., `10%`, `3`).
- This limits how many nodes Karpenter can drain at once during consolidation, expiry-based replacement, or drift reconciliation.
- For advanced disruption scheduling (e.g., no disruptions during business hours), use [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_node_expiry)
- [karpenter_consolidation_enabled](/configuration/rack-parameters/aws/karpenter_consolidation_enabled)
