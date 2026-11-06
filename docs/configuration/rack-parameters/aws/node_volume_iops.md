---
title: "node_volume_iops"
description: "The node_volume_iops AWS rack parameter sets provisioned IOPS on each node's root volume, defaulting to 0, which keeps the gp3 baseline of 3,000."
slug: node_volume_iops
url: /configuration/rack-parameters/aws/node_volume_iops
---

# node_volume_iops

## Description

The `node_volume_iops` parameter sets the provisioned IOPS on the root volume of each node in the cluster.

It reaches every EKS managed node group on the Rack: the system node group, the build node group, every group in [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config), and every group in [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config). They all run gp3 root volumes, so the volume type is never a question on this path.

On a Rack running [Karpenter](/configuration/scaling/karpenter) the value also reaches every Karpenter pool, because [`karpenter_node_volume_iops`](/configuration/rack-parameters/aws/karpenter_node_volume_iops) at its default `0` inherits it. That inheritance renders on gp3 only: with [`karpenter_node_volume_type`](/configuration/rack-parameters/aws/karpenter_node_volume_type) set to `gp2`, `io1` or `io2`, the managed node groups still take the value and the Karpenter workload and build pools do not. Custom pools in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) are judged on their own `volume_type`, which defaults to `gp3`, so they keep receiving the inherited value on a Rack set to another type.

## Default Value

The default value for `node_volume_iops` is `0`, which leaves the AWS gp3 baseline of 3,000 IOPS in place. At `0` Convox renders no `iops` attribute on the launch templates rather than an explicit 3000.

## Setting Parameters

**Changing the value recycles every node in the managed node groups,** so schedule the change. The node groups keep their names and none of them is replaced, and no existing EBS volume is modified: nodes are replaced, and each replacement comes up with a new volume at the new setting.

```bash
$ convox rack params set node_volume_iops=6000 -r rackName
Updating parameters... OK
```

Accepted values are `0`, or 3000 to 80000. Anything else is rejected before any Terraform apply:

```bash
$ convox rack params set node_volume_iops=1500 -r rackName
ERROR: param 'node_volume_iops' must be 0 or between 3000 and 80000 IOPS
```

A value between 1 and 2999 is rejected rather than raised to the baseline, because gp3 takes no provisioned IOPS below 3,000.

Managed node groups recycle one node at a time unless [`node_max_unavailable_percentage`](/configuration/rack-parameters/aws/node_max_unavailable_percentage) is set, which paces the system node group and additional node groups; the build node groups have no pacing setting. Karpenter pools are paced by their disruption budgets. On a large Rack, set `node_max_unavailable_percentage` before changing this parameter.

## The Size Ratio

A gp3 volume takes at most 500 provisioned IOPS per GiB of its own size. A request above that ceiling is lowered to the ceiling rather than rejected, so the value you set and the value a node runs can differ.

At the default [`node_disk`](/configuration/rack-parameters/aws/node_disk) of 20 GiB the ceiling is 10,000 IOPS. Setting `node_volume_iops=40000` on that Rack applies 10,000.

| `node_disk` | IOPS ceiling |
|-------------|--------------|
| 20 GiB (the default) | 10,000 |
| 50 GiB | 25,000 |
| 100 GiB | 50,000 |
| 160 GiB or more | 80,000 (the parameter maximum) |

Each volume is measured against its own size rather than the Rack's. A node group that sets its own `disk` in [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config) gets the ceiling for that size: on a Rack with `node_disk=200` and an additional node group at `"disk": 20`, `node_volume_iops=80000` applies 80,000 to the system and build launch templates and 10,000 to that group's.

Throughput follows from the IOPS, at a quarter of them, so raising this value also raises the ceiling on [`node_volume_throughput`](/configuration/rack-parameters/aws/node_volume_throughput).

## Lowering the Value

Setting `node_volume_iops` back to `0` does not lower the managed node groups. At `0` Convox renders no `iops` attribute, and a launch template keeps the last value applied to it. To lower provisioned IOPS on a managed node group, set a lower number rather than `0`.

Karpenter pools work the other way while [`karpenter_node_volume_iops`](/configuration/rack-parameters/aws/karpenter_node_volume_iops) is also `0`. Returning both to `0` drops the field from the EC2NodeClass and puts those pools back on the gp3 baseline of 3,000 IOPS. A non-zero `karpenter_node_volume_iops` keeps applying its own value whatever this parameter holds.

Downgrading below `3.25.7` splits the same way. Parameter reconciliation deletes the parameter from the stored values before the apply runs and prints `NOTICE: removing parameters not supported by version <version>: node_volume_iops` on stderr, the managed node group launch templates keep the last `iops` applied to them, and the Karpenter EC2NodeClasses lose the field, so those pools return to the gp3 baseline. On a Rack managed through the Console the value stays in the Console's stored parameters and is applied again when you upgrade back to `3.25.7` or later. On a self-managed Rack, set it again after the upgrade.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.7` or later. Setting it also requires a `convox` CLI at `3.25.7` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

- **The parameter is not clearable.** `convox rack params set node_volume_iops=` is rejected with `param 'node_volume_iops' requires an explicit value (omit to keep current)`. Pass `0` rather than an empty value.
- **Enabling Karpenter:** on a Rack with [`high_availability`](/configuration/rack-parameters/aws/high_availability) set to `false`, this parameter cannot be set in the same call as `karpenter_enabled=true`. Set it first, wait for the update to finish, then enable Karpenter.
- **Parameter groups:** `nodes` and `storage`. `convox rack params -g storage -r rackName` lists it next to the other volume settings.
- `convox rack params` lists stored values only, so `node_volume_iops` does not appear in that output until you set it.

## See Also

- [node_volume_throughput](/configuration/rack-parameters/aws/node_volume_throughput) for the throughput on the same volume, which is capped at a quarter of these IOPS
- [node_disk](/configuration/rack-parameters/aws/node_disk) for the volume size that sets the 500-per-GiB ceiling
- [karpenter_node_volume_iops](/configuration/rack-parameters/aws/karpenter_node_volume_iops) for overriding this value on Karpenter pools
- [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config) for the per-group disk sizes each volume is measured against
