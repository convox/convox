---
title: "karpenter_node_volume_throughput"
description: "The karpenter_node_volume_throughput AWS rack parameter sets provisioned MiB/s for Karpenter node root volumes, inheriting node_volume_throughput when 0."
slug: karpenter_node_volume_throughput
url: /configuration/rack-parameters/aws/karpenter_node_volume_throughput
---

# karpenter_node_volume_throughput

## Description

The `karpenter_node_volume_throughput` parameter sets the provisioned throughput, in MiB/s, on the root volume of [Karpenter](/configuration/scaling/karpenter)-provisioned nodes. Throughput is the bandwidth a node has for reading and writing that volume, which is what a large container image pull is bound by. The parameter does not reach the EKS managed node groups, which follow [`node_volume_throughput`](/configuration/rack-parameters/aws/node_volume_throughput).

## Default Value

The default value is `0`, which inherits the Rack's [`node_volume_throughput`](/configuration/rack-parameters/aws/node_volume_throughput) value. With both at `0`, Karpenter nodes run the AWS gp3 baseline of 125 MiB/s.

Returning the parameter to `0` drops the field from the EC2NodeClass and puts the pools back on the inherited value, or on the gp3 baseline when `node_volume_throughput` is also `0`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_volume_throughput=600 -r rackName
Updating parameters... OK
```

Accepted values are `0`, or 125 to 1000. Anything else is rejected before any Terraform apply:

```bash
$ convox rack params set karpenter_node_volume_throughput=2000 -r rackName
ERROR: param 'karpenter_node_volume_throughput' must be 0 or between 125 and 1000 MiB/s
```

Convox applies a ceiling of 1,000 MiB/s on the Karpenter pools and the managed node groups alike, so one number covers every node on the Rack.

Changing the value rolls Karpenter nodes, paced by each pool's disruption budget.

## Which Pools Receive It

The workload NodePool and the build NodePool take the value directly. Custom pools in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) inherit it unless the entry sets its own `volume_throughput`.

Setting `karpenter_config.ec2NodeClass.blockDeviceMappings` replaces the generated block device list on the workload pool, which takes that pool out of this parameter. See [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config).

## The IOPS Ratio

A gp3 volume takes at most a quarter of its provisioned IOPS as throughput in MiB/s, and each pool is measured against its own IOPS. A request above that ceiling is lowered to the ceiling rather than rejected, so the value you set and the value a node runs can differ. The floor when the parameter is set is 125.

With [`karpenter_node_volume_iops`](/configuration/rack-parameters/aws/karpenter_node_volume_iops) and [`node_volume_iops`](/configuration/rack-parameters/aws/node_volume_iops) both unset, the ratio runs against the gp3 baseline of 3,000 IOPS and the ceiling is 750 MiB/s. Setting `karpenter_node_volume_throughput=1000` on its own applies 750. Reaching 1,000 MiB/s needs 4,000 IOPS on the same pool, either already applied or set in the same call:

```bash
$ convox rack params set karpenter_node_volume_iops=4000 karpenter_node_volume_throughput=1000 -r rackName
Updating parameters... OK
```

| Effective IOPS on the pool | Throughput ceiling |
|----------------------------|--------------------|
| unset (the gp3 baseline of 3,000) | 750 MiB/s |
| 3000 | 750 MiB/s |
| 4000 or higher | 1,000 MiB/s (the parameter maximum) |

IOPS are themselves capped at 500 per GiB of the pool's own disk, so a pool too small to carry the IOPS is also too small to carry the throughput. See [`karpenter_node_disk`](/configuration/rack-parameters/aws/karpenter_node_disk).

## gp3 Only

AWS takes provisioned throughput on gp3 volumes only, so Convox renders the field on gp3 only. With [`karpenter_node_volume_type`](/configuration/rack-parameters/aws/karpenter_node_volume_type) set to `gp2`, `io1` or `io2`, neither this parameter nor a value inherited from `node_volume_throughput` reaches the workload or build pool.

Setting it while `karpenter_node_volume_type` is not `gp3` is rejected before apply:

```bash
$ convox rack params set karpenter_node_volume_throughput=600 -r rackName
ERROR: param 'karpenter_node_volume_throughput' requires karpenter_node_volume_type=gp3 (currently io2).
  AWS takes these fields on gp3 only, so Convox does not render them on any other type.
  To raise one custom nodepool that does run gp3, set its volume_iops or volume_throughput
  in additional_karpenter_nodepools_config instead
```

Moving `karpenter_node_volume_type` off `gp3` while a value is already stored is accepted, and prints a line to stderr:

```text
WARNING: karpenter_node_volume_throughput no longer applies to the Karpenter workload and build pools now that karpenter_node_volume_type is io2. Custom nodepools that set volume_type gp3 still receive it.
```

A custom pool is judged on its own `volume_type`, which defaults to `gp3`, so a pool that sets nothing keeps receiving provisioned throughput on a Rack set to another type.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.7` or later. Setting it also requires a `convox` CLI at `3.25.7` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

- **600 MiB/s is the floor for fast image pull.** On a Karpenter Rack, [`fast_image_pull_enable`](/configuration/rack-parameters/aws/fast_image_pull_enable) checks the workload pool, the build pool and every gp3 custom pool separately, each against its own effective throughput, and refuses the change while any of them is below 600.
- **The parameter is not clearable.** `convox rack params set karpenter_node_volume_throughput=` is rejected with `param 'karpenter_node_volume_throughput' requires an explicit value (omit to keep current)`. Pass `0` rather than an empty value.
- **Enabling Karpenter:** on a Rack with [`high_availability`](/configuration/rack-parameters/aws/high_availability) set to `false`, this parameter cannot be set in the same call as `karpenter_enabled=true`. Set it first, wait for the update to finish, then enable Karpenter.
- **Parameter groups:** `karpenter` and `storage`. `convox rack params -g storage -r rackName` lists it next to the other volume settings.
- `convox rack params` lists stored values only, so `karpenter_node_volume_throughput` does not appear in that output until you set it.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_volume_iops](/configuration/rack-parameters/aws/karpenter_node_volume_iops) for the provisioned IOPS this value is capped against
- [karpenter_node_volume_type](/configuration/rack-parameters/aws/karpenter_node_volume_type) for the volume type that gates this parameter
- [node_volume_throughput](/configuration/rack-parameters/aws/node_volume_throughput) for the Rack-wide value this parameter inherits at `0`
- [fast_image_pull_enable](/configuration/rack-parameters/aws/fast_image_pull_enable) for parallel image pull and unpack, which needs 600 MiB/s on every pool first
