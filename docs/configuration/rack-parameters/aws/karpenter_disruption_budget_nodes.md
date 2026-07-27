---
title: "karpenter_disruption_budget_nodes"
description: "The karpenter_disruption_budget_nodes AWS rack parameter caps how many Karpenter workload nodes can be disrupted at once during consolidation, defaulting to 10%."
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
Updating parameters... OK
```

## Additional Information

- **Validation:** Must be a node count or a percentage from `0%` to `100%`, matching the regex `^((100|[0-9]{1,2})%|[0-9]+)$` (e.g., `10%`, `3`). This matches the Karpenter NodePool validation, so a percentage above `100%` is rejected by the CLI instead of failing a Rack update.
- This limits how many nodes Karpenter can drain at once during consolidation and drift reconciliation. It does not limit node expiry: nodes that reach [`karpenter_node_expiry`](/configuration/rack-parameters/aws/karpenter_node_expiry) are removed regardless of the budget.
- This parameter applies to the workload NodePool only. The build NodePool has its own cap, [`karpenter_build_disruption_budget_nodes`](/configuration/rack-parameters/aws/karpenter_build_disruption_budget_nodes).
- For advanced disruption scheduling (e.g., no disruptions during business hours), use [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_build_disruption_budget_nodes](/configuration/rack-parameters/aws/karpenter_build_disruption_budget_nodes) for the same cap on the build NodePool
- [karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_node_expiry)
- [karpenter_consolidation_enabled](/configuration/rack-parameters/aws/karpenter_consolidation_enabled)
