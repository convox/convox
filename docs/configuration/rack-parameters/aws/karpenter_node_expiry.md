---
title: "karpenter_node_expiry"
slug: karpenter_node_expiry
url: /configuration/rack-parameters/aws/karpenter_node_expiry
---

# karpenter_node_expiry

## Description

The `karpenter_node_expiry` parameter sets the maximum lifetime for [Karpenter](/configuration/scaling/karpenter) workload nodes before they are automatically replaced. This keeps your fleet on current AMIs without manual intervention.

## Default Value

The default value is `720h` (30 days).

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_expiry=336h -r rackName
Setting parameters... OK
```

To disable automatic replacement:

```bash
$ convox rack params set karpenter_node_expiry=Never -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be a duration in hours (e.g., `720h`, `336h`) or `Never`.
- When a node reaches its expiry, Karpenter gracefully drains pods before terminating the node, subject to [`karpenter_disruption_budget_nodes`](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_disruption_budget_nodes](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes)
