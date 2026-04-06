---
title: "karpenter_build_node_labels"
slug: karpenter_build_node_labels
url: /configuration/rack-parameters/aws/karpenter_build_node_labels
---

# karpenter_build_node_labels

## Description

The `karpenter_build_node_labels` parameter adds custom Kubernetes labels to [Karpenter](/configuration/scaling/karpenter) build nodes. These labels are applied alongside the default `convox-build=true` and `convox.io/nodepool=build` labels.

## Default Value

The default value is empty (no custom labels).

## Setting the Parameter

```bash
$ convox rack params set karpenter_build_node_labels=environment=build,team=platform -r rackName
Setting parameters... OK
```

## Additional Information

- **Format:** Comma-separated `key=value` pairs.
- **Validation:** Label keys and values must not contain double quotes. The `convox-build` and `convox.io/nodepool` label keys are reserved and cannot be overridden.
- The build NodePool is only created when [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_labels](/configuration/rack-parameters/aws/karpenter_node_labels) for workload node labels
