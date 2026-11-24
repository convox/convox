---
title: "scale_down_utilization_percentage"
description: "The scale_down_utilization_percentage AWS rack parameter sets the Cluster Autoscaler scale-down utilization threshold, defaulting to 50."
slug: scale_down_utilization_percentage
url: /configuration/rack-parameters/aws/scale_down_utilization_percentage
---

# scale_down_utilization_percentage

## Description
The `scale_down_utilization_percentage` parameter sets the Cluster Autoscaler's scale-down utilization threshold, as a whole percentage. The autoscaler considers removing a node only when the CPU and the memory requested by the pods on it are both below this percentage of the node's allocatable capacity, and those pods can be scheduled elsewhere.

It is passed to Cluster Autoscaler as `--scale-down-utilization-threshold` (for example, `65` becomes `0.65`).

## Default Value
The default value for `scale_down_utilization_percentage` is `50`, the Cluster Autoscaler default. At `50` the flag is not passed, so the autoscaler Deployment is unchanged.

## Use Cases
- **Tighter packing**: the scheduler spreads pods across nodes, so after pod requests are cut, many nodes can sit between 50% and 65% requested. None of them is drained at the default, even when the pods would fit on fewer nodes. Raising the value to `60` or `65` lets the autoscaler remove them.
- **Stability**: lowering the value makes scale-down rarer, which reduces pod moves on Racks with long-running or slow-starting workloads.

## Setting Parameters
```bash
$ convox rack params set scale_down_utilization_percentage=65 -r rackName
Updating parameters... OK
```

The value must be an integer between 1 and 100.

To return to the Cluster Autoscaler default:
```bash
$ convox rack params set scale_down_utilization_percentage=50 -r rackName
Updating parameters... OK
```

## Additional Information
- A higher value moves pods more often: each scale-down evicts the pods on the removed node. Pod Disruption Budgets still apply; see [pdb_default_min_available_percentage](/configuration/rack-parameters/aws/pdb_default_min_available_percentage).
- On a Rack with [`karpenter_enabled`](/configuration/rack-parameters/aws/karpenter_enabled) set to `true`, Cluster Autoscaler manages only the additional node groups, so this parameter applies only to them. Karpenter pools use their own consolidation settings.
- Changing this parameter restarts the Cluster Autoscaler pod. It does not replace nodes.

## See Also
- [node_max_unavailable_percentage](/configuration/rack-parameters/aws/node_max_unavailable_percentage) for node group update pacing
- [pdb_default_min_available_percentage](/configuration/rack-parameters/aws/pdb_default_min_available_percentage) for pod-level disruption budgets
