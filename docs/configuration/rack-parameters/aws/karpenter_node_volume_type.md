---
title: "karpenter_node_volume_type"
description: "The karpenter_node_volume_type AWS rack parameter sets the EBS volume type for the Karpenter workload and build node pools, defaulting to gp3."
slug: karpenter_node_volume_type
url: /configuration/rack-parameters/aws/karpenter_node_volume_type
---

# karpenter_node_volume_type

## Description

The `karpenter_node_volume_type` parameter sets the EBS volume type for the [Karpenter](/configuration/scaling/karpenter) workload and build node pools. Custom pools set their own `volume_type` in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config), defaulting to `gp3`, and this parameter does not reach them.

## Default Value

The default value is `gp3`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_volume_type=io1 -r rackName
Updating parameters... OK
```

## Additional Information

- **Validation:** Must be `gp2`, `gp3`, `io1`, or `io2`.
- **`gp3` is required for provisioned performance.** [`karpenter_node_volume_iops`](/configuration/rack-parameters/aws/karpenter_node_volume_iops) and [`karpenter_node_volume_throughput`](/configuration/rack-parameters/aws/karpenter_node_volume_throughput) render on `gp3` only, and that includes a value inherited from [`node_volume_iops`](/configuration/rack-parameters/aws/node_volume_iops) or [`node_volume_throughput`](/configuration/rack-parameters/aws/node_volume_throughput). Setting either one while this parameter is not `gp3` is rejected before apply. Moving this parameter off `gp3` while a value is already stored is accepted with a warning that it no longer applies to the workload and build pools.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_disk](/configuration/rack-parameters/aws/karpenter_node_disk)
- [karpenter_node_volume_iops](/configuration/rack-parameters/aws/karpenter_node_volume_iops) for provisioned IOPS on these volumes
- [karpenter_node_volume_throughput](/configuration/rack-parameters/aws/karpenter_node_volume_throughput) for provisioned throughput on these volumes
