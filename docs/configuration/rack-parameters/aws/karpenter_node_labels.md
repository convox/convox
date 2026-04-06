---
title: "karpenter_node_labels"
slug: karpenter_node_labels
url: /configuration/rack-parameters/aws/karpenter_node_labels
---

# karpenter_node_labels

## Description

The `karpenter_node_labels` parameter adds custom Kubernetes labels to [Karpenter](/configuration/scaling/karpenter) workload nodes. These labels are applied alongside the default `convox.io/nodepool=workload` label.

## Default Value

The default value is empty (no custom labels).

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_labels=environment=production,team=platform -r rackName
Setting parameters... OK
```

## Additional Information

- **Format:** Comma-separated `key=value` pairs.
- **Validation:** Label keys and values must not contain double quotes. The `convox.io/nodepool` label key is reserved and cannot be overridden.
- Use these labels with `nodeSelectorLabels` in `convox.yml` to target Services to Karpenter workload nodes.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_taints](/configuration/rack-parameters/aws/karpenter_node_taints)
- [Workload Placement](/configuration/scaling/workload-placement) for node targeting with `nodeSelectorLabels`
