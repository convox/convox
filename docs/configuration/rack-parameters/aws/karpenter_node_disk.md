---
title: "karpenter_node_disk"
slug: karpenter_node_disk
url: /configuration/rack-parameters/aws/karpenter_node_disk
---

# karpenter_node_disk

## Description

The `karpenter_node_disk` parameter sets the EBS volume size in GiB for [Karpenter](/configuration/scaling/karpenter)-provisioned workload nodes.

## Default Value

The default value is `0` (inherits the Rack's [`node_disk`](/configuration/rack-parameters/aws/node_disk) value).

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_disk=100 -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be a non-negative integer.
- When set to `0`, Karpenter nodes use the same disk size as the Rack's primary `node_disk` parameter.
- For advanced EBS configuration (custom block device mappings, IOPS), use [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_volume_type](/configuration/rack-parameters/aws/karpenter_node_volume_type)
- [node_disk](/configuration/rack-parameters/aws/node_disk) for primary node disk size
