---
title: "karpenter_node_volume_iops"
description: "The karpenter_node_volume_iops AWS rack parameter sets provisioned IOPS for Karpenter node root volumes, inheriting node_volume_iops when 0. gp3 only."
slug: karpenter_node_volume_iops
url: /configuration/rack-parameters/aws/karpenter_node_volume_iops
---

# karpenter_node_volume_iops

## Description

The `karpenter_node_volume_iops` parameter sets the provisioned IOPS on the root volume of [Karpenter](/configuration/scaling/karpenter)-provisioned nodes. It does not reach the EKS managed node groups, which follow [`node_volume_iops`](/configuration/rack-parameters/aws/node_volume_iops).

## Default Value

The default value is `0`, which inherits the Rack's [`node_volume_iops`](/configuration/rack-parameters/aws/node_volume_iops) value, in the same way [`karpenter_node_disk`](/configuration/rack-parameters/aws/karpenter_node_disk) inherits [`node_disk`](/configuration/rack-parameters/aws/node_disk). With both at `0`, Karpenter nodes run the AWS gp3 baseline of 3,000 IOPS.

Returning the parameter to `0` drops the field from the EC2NodeClass and puts the pools back on the inherited value, or on the gp3 baseline when `node_volume_iops` is also `0`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_volume_iops=6000 -r rackName
Updating parameters... OK
```

Accepted values are `0`, or 3000 to 80000. Anything else is rejected before any Terraform apply:

```bash
$ convox rack params set karpenter_node_volume_iops=1500 -r rackName
ERROR: param 'karpenter_node_volume_iops' must be 0 or between 3000 and 80000 IOPS
```

Changing the value rolls Karpenter nodes, paced by each pool's disruption budget, because the block device mapping is part of the EC2NodeClass drift hash.

## Which Pools Receive It

The workload NodePool and the build NodePool take the value directly. Custom pools in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) inherit it unless the entry sets its own `volume_iops`.

Each pool is clamped at 500 IOPS per GiB of its own disk, with a floor of 3,000 when set. For the workload and build pools that disk is [`karpenter_node_disk`](/configuration/rack-parameters/aws/karpenter_node_disk), or `node_disk` when `karpenter_node_disk` is `0`; for a custom pool it is that pool's `disk`. At the default 20 GiB the ceiling is 10,000 IOPS, and a higher request is lowered to it rather than rejected. Raising the disk size raises the ceiling with it.

Setting `karpenter_config.ec2NodeClass.blockDeviceMappings` replaces the generated block device list on the workload pool, which takes that pool out of this parameter. See [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config).

## gp3 Only

AWS takes provisioned IOPS on gp3 volumes only, so Convox renders the field on gp3 only. With [`karpenter_node_volume_type`](/configuration/rack-parameters/aws/karpenter_node_volume_type) set to `gp2`, `io1` or `io2`, neither this parameter nor a value inherited from `node_volume_iops` reaches the workload or build pool.

Setting it while `karpenter_node_volume_type` is not `gp3` is rejected before apply:

```bash
$ convox rack params set karpenter_node_volume_iops=6000 -r rackName
ERROR: param 'karpenter_node_volume_iops' requires karpenter_node_volume_type=gp3 (currently io2).
  AWS takes these fields on gp3 only, so Convox does not render them on any other type.
  To raise one custom nodepool that does run gp3, set its volume_iops or volume_throughput
  in additional_karpenter_nodepools_config instead
```

Moving `karpenter_node_volume_type` off `gp3` while a value is already stored is accepted, and prints a line to stderr:

```text
WARNING: karpenter_node_volume_iops no longer applies to the Karpenter workload and build pools now that karpenter_node_volume_type is io2. Custom nodepools that set volume_type gp3 still receive it.
```

A custom pool is judged on its own `volume_type`, which defaults to `gp3`, so a pool that sets nothing keeps receiving provisioned IOPS on a Rack set to another type.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.7` or later. Setting it also requires a `convox` CLI at `3.25.7` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

- **The parameter is not clearable.** `convox rack params set karpenter_node_volume_iops=` is rejected with `param 'karpenter_node_volume_iops' requires an explicit value (omit to keep current)`. Pass `0` rather than an empty value.
- **Enabling Karpenter:** on a Rack with [`high_availability`](/configuration/rack-parameters/aws/high_availability) set to `false`, this parameter cannot be set in the same call as `karpenter_enabled=true`. Set it first, wait for the update to finish, then enable Karpenter.
- **Parameter groups:** `karpenter` and `storage`. `convox rack params -g storage -r rackName` lists it next to the other volume settings.
- `convox rack params` lists stored values only, so `karpenter_node_volume_iops` does not appear in that output until you set it.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_volume_throughput](/configuration/rack-parameters/aws/karpenter_node_volume_throughput) for the throughput on the same volume, which is capped at a quarter of these IOPS
- [karpenter_node_volume_type](/configuration/rack-parameters/aws/karpenter_node_volume_type) for the volume type that gates this parameter
- [karpenter_node_disk](/configuration/rack-parameters/aws/karpenter_node_disk) for the volume size that sets the 500-per-GiB ceiling
- [node_volume_iops](/configuration/rack-parameters/aws/node_volume_iops) for the Rack-wide value this parameter inherits at `0`
