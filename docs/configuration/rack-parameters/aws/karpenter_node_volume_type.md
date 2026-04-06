---
title: "karpenter_node_volume_type"
slug: karpenter_node_volume_type
url: /configuration/rack-parameters/aws/karpenter_node_volume_type
---

# karpenter_node_volume_type

## Description

The `karpenter_node_volume_type` parameter sets the EBS volume type for [Karpenter](/configuration/scaling/karpenter)-provisioned workload nodes.

## Default Value

The default value is `gp3`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_volume_type=io1 -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be `gp2`, `gp3`, `io1`, or `io2`.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_disk](/configuration/rack-parameters/aws/karpenter_node_disk)
