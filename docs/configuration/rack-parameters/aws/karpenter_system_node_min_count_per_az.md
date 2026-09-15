---
title: "karpenter_system_node_min_count_per_az"
description: "The karpenter_system_node_min_count_per_az AWS rack parameter sets the minimum number of system nodes each availability zone runs while Karpenter is enabled, defaulting to 1."
slug: karpenter_system_node_min_count_per_az
url: /configuration/rack-parameters/aws/karpenter_system_node_min_count_per_az
---

# karpenter_system_node_min_count_per_az

## Description

The `karpenter_system_node_min_count_per_az` parameter sets the minimum number of system nodes each availability zone runs while [`karpenter_enabled`](/configuration/rack-parameters/aws/karpenter_enabled) is `true`. It has no effect when Karpenter is off, where each system node group's minimum is one node, or [`min_on_demand_count`](/configuration/rack-parameters/aws/min_on_demand_count) for the on-demand group on a [`node_capacity_type`](/configuration/rack-parameters/aws/node_capacity_type) of `mixed`, and Cluster Autoscaler scales them.

## Default Value

The default value is `1`.

`convox rack params` lists stored values only. This parameter does not appear in that output until you set it, so an absent entry means the default is in effect.

## Use Cases

- **Headroom for Rack components**: system nodes carry the Rack API, the Router, CoreDNS, the CSI controllers and the rest of the Rack's own workloads. One node per zone covers most Racks and not a large one.
- **Spreading the Rack's own workloads**: raising the count gives the scheduler more system nodes to place those Pods across, in every zone rather than one.

## How the Value Is Counted

Convox runs one system node group per availability zone: three on a [`high_availability`](/configuration/rack-parameters/aws/high_availability) Rack and one otherwise. The value lands on each group.

| Rack | System node groups | Nodes at `=1` | Nodes at `=2` |
|------|--------------------|---------------|---------------|
| `high_availability=true` | 3, one per zone | 3 | 6 |
| `high_availability=false` | 1 | 1 | 2 |

This is the only node count parameter on an AWS Rack whose value is not a Rack total. [`build_node_min_count`](/configuration/rack-parameters/aws/build_node_min_count) and [`min_on_demand_count`](/configuration/rack-parameters/aws/min_on_demand_count) each land on a single node group.

## Setting the Parameter

**A mistyped value does not undo itself.** Raising the parameter adds nodes and lowering it removes none, and the value lands on every system node group: `karpenter_system_node_min_count_per_az=100` on a [`high_availability`](/configuration/rack-parameters/aws/high_availability) Rack starts 300 on-demand instances. A value above `10` also blocks a downgrade below `3.25.7`. See [Raising and Lowering](#raising-and-lowering).

```bash
$ convox rack params set karpenter_system_node_min_count_per_az=2 -r rackName
Updating parameters... OK
```

A value outside the range is rejected rather than clamped:

```text
karpenter_system_node_min_count_per_az must be an integer from 1 to 100
```

The parameter is not clearable. Return to the default by setting it back to `1`; an empty value is rejected with `param 'karpenter_system_node_min_count_per_az' requires an explicit value (omit to keep current)`.

Setting the value while `karpenter_enabled` is `false` is accepted and changes nothing. The value is range checked again when Karpenter is enabled.

## Raising and Lowering

Raising the parameter adds nodes. Convox raises each system node group's running size to the new minimum before the apply and waits for the new nodes, so the update takes longer when the jump is large. It prints one `NOTICE` per group:

```text
NOTICE: raising node group my-rack-us-east-2a-0a1b2c3d4e5f60718 desired size to 2 before apply
```

Lowering the parameter removes nothing. It lowers each group's minimum and stops there. Nothing scales these groups down while Karpenter is enabled: Cluster Autoscaler targets additional node groups only, and Convox never lowers a node group's running size. Remove the nodes by replacing the groups with a [`node_disk`](/configuration/rack-parameters/aws/node_disk) or [`node_type`](/configuration/rack-parameters/aws/node_type) change, or by lowering each group's desired size yourself with `aws eks update-nodegroup-config`.

## Downgrading Below 3.25.7

Rack versions before `3.25.7` cap each system node group at 10 nodes while Karpenter is enabled. A Rack that has ever applied a value above `10` keeps that many nodes running, so the older version's maximum sits below the live count and the downgrade apply fails:

```text
InvalidParameterException: desired capacity 11 can't be greater than max size 10
```

Lowering the parameter does not clear this. It lowers the minimum and leaves the running nodes in place. Replace the system node groups instead: set the parameter to `10` or less, wait for that update to finish, then change [`node_disk`](/configuration/rack-parameters/aws/node_disk) or [`node_type`](/configuration/rack-parameters/aws/node_type) to a different value. Either one replaces the system node groups and creates them again at the configured count, which rolls every system node and takes as long as any node group replacement.

The build node group is a second blocker on the same downgrade. Versions before `3.25.7` cap it at one node while Karpenter is enabled, so a Rack running more than one build node has to replace that group too. A `node_disk` change replaces it along with the system node groups. A `node_type` change replaces it only when [`build_node_type`](/configuration/rack-parameters/aws/build_node_type) is unset, because the build node group then takes its instance type from `node_type`; where `build_node_type` is set, change that to a different instance type. The downgrade applies once both groups are within the older maximums. Changing `node_capacity_type` does not work, because Karpenter already forces it to `ON_DEMAND`.

A Rack can reach this state without the parameter. Cluster Autoscaler can take a system node group past ten nodes before Karpenter is enabled. The restriction is on the running node counts, not on the parameter.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.7` or later. Setting it also requires a `convox` CLI at `3.25.7` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

Raising `karpenter_system_node_min_count_per_az` above the number of nodes a system node group is currently running requires a `convox` CLI at `3.25.7` or newer performing the apply. Earlier versions fail the apply with `InvalidParameterException: Minimum capacity 2 can't be greater than desired size 1` and roll the value back. Setting the value at install, and lowering it, work on any version that accepts the parameter.

This is not a Rack version requirement. The handling lives in the `convox` binary performing the apply, which for a Console-managed Rack is the CLI bundled in the Console deploy rather than the CLI on your machine. See [Raising a node group minimum fails EKS validation](/help/troubleshooting#raising-a-node-group-minimum-fails-eks-validation) and [CLI Rack Management](/management/cli-rack-management).

- **Validation:** must be an integer from 1 to 100.
- **Roll length.** At the default [`node_max_unavailable_percentage`](/configuration/rack-parameters/aws/node_max_unavailable_percentage) of `0`, EKS replaces one node per group at a time. Raising the system node count lengthens every roll of those groups in proportion, on a Kubernetes version bump, an AMI change, or any launch template change.
- **Parameter groups:** listed under the `karpenter` and `scaling` groups (`convox rack params -g karpenter -r rackName`).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [System Node Behavior](/configuration/scaling/karpenter#system-node-behavior) for what runs on system nodes
- [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled) to turn Karpenter on
- [high_availability](/configuration/rack-parameters/aws/high_availability) for how many availability zones the Rack spans
- [node_type](/configuration/rack-parameters/aws/node_type) for the system node instance type
- [node_disk](/configuration/rack-parameters/aws/node_disk) for the system node disk size
- [node_max_unavailable_percentage](/configuration/rack-parameters/aws/node_max_unavailable_percentage) for how many nodes a group replaces at once
- [min_on_demand_count](/configuration/rack-parameters/aws/min_on_demand_count) for the on-demand minimum on a `mixed` Rack with Karpenter off
- [CLI Rack Management](/management/cli-rack-management) for which `convox` binary performs a Rack apply
