---
title: "node_volume_throughput"
description: "The node_volume_throughput AWS rack parameter sets provisioned throughput in MiB/s on each node's root volume, defaulting to 0, which keeps the gp3 baseline of 125."
slug: node_volume_throughput
url: /configuration/rack-parameters/aws/node_volume_throughput
---

# node_volume_throughput

## Description

The `node_volume_throughput` parameter sets the provisioned throughput, in MiB/s, on the root volume of each node in the cluster. Throughput is the bandwidth a node has for reading and writing that volume, which is what a large container image pull is bound by.

It reaches every EKS managed node group on the Rack: the system node group, the build node group, every group in [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config), and every group in [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config). They all run gp3 root volumes.

On a Rack running [Karpenter](/configuration/scaling/karpenter) the value also reaches every Karpenter pool, because [`karpenter_node_volume_throughput`](/configuration/rack-parameters/aws/karpenter_node_volume_throughput) at its default `0` inherits it. That inheritance renders on gp3 only: with [`karpenter_node_volume_type`](/configuration/rack-parameters/aws/karpenter_node_volume_type) set to `gp2`, `io1` or `io2`, the managed node groups still take the value and the Karpenter workload and build pools do not. Custom pools in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) are judged on their own `volume_type`, which defaults to `gp3`, so they keep receiving the inherited value on a Rack set to another type.

## Default Value

The default value for `node_volume_throughput` is `0`, which leaves the AWS gp3 baseline of 125 MiB/s in place. At `0` Convox renders no `throughput` attribute on the launch templates rather than an explicit 125.

## Setting Parameters

**Changing the value recycles every node in the managed node groups,** so schedule the change. The node groups keep their names and none of them is replaced, and no existing EBS volume is modified: nodes are replaced, and each replacement comes up with a new volume at the new setting.

```bash
$ convox rack params set node_volume_throughput=600 -r rackName
Updating parameters... OK
```

Accepted values are `0`, or 125 to 1000. Anything else is rejected before any Terraform apply:

```bash
$ convox rack params set node_volume_throughput=2000 -r rackName
ERROR: param 'node_volume_throughput' must be 0 or between 125 and 1000 MiB/s
```

Convox applies a ceiling of 1,000 MiB/s on both the managed node groups and the Karpenter pools, so one number covers every node on the Rack.

Managed node groups recycle one node at a time unless [`node_max_unavailable_percentage`](/configuration/rack-parameters/aws/node_max_unavailable_percentage) is set, which paces the system node group and additional node groups; the build node groups have no pacing setting. Karpenter pools are paced by their disruption budgets.

## The IOPS Ratio

A gp3 volume takes at most a quarter of its provisioned IOPS as throughput in MiB/s. A request above that ceiling is lowered to the ceiling rather than rejected, so the value you set and the value a node runs can differ.

With [`node_volume_iops`](/configuration/rack-parameters/aws/node_volume_iops) unset, the ratio runs against the gp3 baseline of 3,000 IOPS and the ceiling is 750 MiB/s. Setting `node_volume_throughput=1000` on its own applies 750. Reaching 1,000 MiB/s needs `node_volume_iops` of at least 4,000, either already applied or set in the same call:

```bash
$ convox rack params set node_volume_iops=4000 node_volume_throughput=1000 -r rackName
Updating parameters... OK
```

| `node_volume_iops` | Throughput ceiling |
|--------------------|--------------------|
| unset (the gp3 baseline of 3,000) | 750 MiB/s |
| 3000 | 750 MiB/s |
| 4000 or higher | 1,000 MiB/s (the parameter maximum) |

IOPS are themselves capped at 500 per GiB of the volume's own size, so a volume too small to carry the IOPS is also too small to carry the throughput. See [`node_disk`](/configuration/rack-parameters/aws/node_disk).

## Lowering the Value

Setting `node_volume_throughput` back to `0` does not lower the managed node groups. At `0` Convox renders no `throughput` attribute, and a launch template keeps the last value applied to it. To lower provisioned throughput on a managed node group, set a lower number rather than `0`.

Karpenter pools work the other way. Returning to `0` drops the field from the EC2NodeClass and puts those pools back on the gp3 baseline of 125 MiB/s.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.7` or later. Setting it also requires a `convox` CLI at `3.25.7` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

- **600 MiB/s is the floor for fast image pull.** [`fast_image_pull_enable`](/configuration/rack-parameters/aws/fast_image_pull_enable) is refused below that, and the check runs in both directions: lowering `node_volume_throughput` back under 600 while the gate is on is refused with the same message and the same `--force` escape.
- **The parameter is not clearable.** `convox rack params set node_volume_throughput=` is rejected with `param 'node_volume_throughput' requires an explicit value (omit to keep current)`. Pass `0` rather than an empty value.
- **Enabling Karpenter:** on a Rack with [`high_availability`](/configuration/rack-parameters/aws/high_availability) set to `false`, this parameter cannot be set in the same call as `karpenter_enabled=true`. Set it first, wait for the update to finish, then enable Karpenter.
- **Parameter groups:** `nodes` and `storage`. `convox rack params -g storage -r rackName` lists it next to the other volume settings.
- `convox rack params` lists stored values only, so `node_volume_throughput` does not appear in that output until you set it.

## See Also

- [node_volume_iops](/configuration/rack-parameters/aws/node_volume_iops) for the provisioned IOPS this value is capped against
- [fast_image_pull_enable](/configuration/rack-parameters/aws/fast_image_pull_enable) for parallel image pull and unpack, which needs 600 MiB/s here first
- [karpenter_node_volume_throughput](/configuration/rack-parameters/aws/karpenter_node_volume_throughput) for overriding this value on Karpenter pools
- [kubelet_registry_pull_qps](/configuration/rack-parameters/aws/kubelet_registry_pull_qps) for the limit on how many pulls may start per second, which is a different problem
