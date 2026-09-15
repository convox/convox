---
title: "karpenter_node_disk"
description: "The karpenter_node_disk AWS rack parameter sets the EBS volume size in GiB for the Karpenter workload and build node pools, inheriting node_disk when 0."
slug: karpenter_node_disk
url: /configuration/rack-parameters/aws/karpenter_node_disk
---

# karpenter_node_disk

## Description

The `karpenter_node_disk` parameter sets the EBS volume size in GiB for the [Karpenter](/configuration/scaling/karpenter) workload and build node pools. A custom pool that sets its own `disk` in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) uses that value instead.

## Default Value

The default value is `0` (inherits the Rack's [`node_disk`](/configuration/rack-parameters/aws/node_disk) value).

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_disk=100 -r rackName
Updating parameters... OK
```

## Additional Information

- **Validation:** Must be a non-negative integer.
- When set to `0`, Karpenter nodes use the same disk size as the Rack's primary `node_disk` parameter.
- **Volume performance:** set [`karpenter_node_volume_iops`](/configuration/rack-parameters/aws/karpenter_node_volume_iops) and [`karpenter_node_volume_throughput`](/configuration/rack-parameters/aws/karpenter_node_volume_throughput) for provisioned IOPS and throughput on these volumes, from Rack version `3.25.7`. Both render only while [`karpenter_node_volume_type`](/configuration/rack-parameters/aws/karpenter_node_volume_type) is `gp3`. IOPS are capped at 500 per GiB of this disk size, so raising `karpenter_node_disk` also raises the ceiling.
- For block device configuration beyond size, type, IOPS and throughput, use [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_volume_type](/configuration/rack-parameters/aws/karpenter_node_volume_type)
- [node_disk](/configuration/rack-parameters/aws/node_disk) for primary node disk size
- [karpenter_node_volume_iops](/configuration/rack-parameters/aws/karpenter_node_volume_iops) and [karpenter_node_volume_throughput](/configuration/rack-parameters/aws/karpenter_node_volume_throughput) for provisioned performance on the same volume
