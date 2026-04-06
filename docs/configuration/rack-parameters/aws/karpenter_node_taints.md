---
title: "karpenter_node_taints"
slug: karpenter_node_taints
url: /configuration/rack-parameters/aws/karpenter_node_taints
---

# karpenter_node_taints

## Description

The `karpenter_node_taints` parameter adds custom Kubernetes taints to [Karpenter](/configuration/scaling/karpenter) workload nodes. Services must have matching `tolerations` in `convox.yml` to be scheduled on tainted nodes.

## Default Value

The default value is empty (no custom taints).

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_taints=dedicated=workload:NoSchedule -r rackName
Setting parameters... OK
```

## Additional Information

- **Format:** Comma-separated `key=value:Effect` or `key:Effect` entries.
- **Validation:** Effect must be `NoSchedule`, `PreferNoSchedule`, or `NoExecute`. Keys and values must not contain double quotes.
- All workload Services must include matching `tolerations` in `convox.yml`, or they will not be scheduled on these nodes.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_labels](/configuration/rack-parameters/aws/karpenter_node_labels)
