---
title: "karpenter_node_taints"
slug: karpenter_node_taints
url: /configuration/rack-parameters/aws/karpenter_node_taints
---

# karpenter_node_taints

## Description

The `karpenter_node_taints` parameter adds custom Kubernetes taints to [Karpenter](/configuration/scaling/karpenter) workload nodes. Taints prevent pods without matching tolerations from scheduling on these nodes.

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
- `convox.yml` does not have a `tolerations` field. For GPU taints (`nvidia.com/gpu`, `amd.com/gpu`), Convox emits a matching toleration (`operator: Exists`, `effect: NoSchedule`) directly on any pod that declares `scale.gpu.count > 0` — no admission controller is required. For non-GPU taints, tolerations must be added through an external mechanism (e.g., a mutating admission webhook) or via the `dedicated-node` convention (set `nodeSelectorLabels.convox.io/label: <value>` to also receive the dedicated-node toleration). See [Using Taints to Protect Nodes](/configuration/scaling/karpenter#using-taints-to-protect-nodes) for details.
- Node-level DaemonSets (fluentd, `aws-node`, `kube-proxy`, etc.) are not affected by custom taints — they use broad tolerations and will continue to run on tainted nodes.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_labels](/configuration/rack-parameters/aws/karpenter_node_labels)
