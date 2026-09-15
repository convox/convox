---
title: "node_type"
description: "The node_type AWS rack parameter sets the EC2 instance type for cluster nodes, or a list of types to fall back through, fixing their compute, memory, and CPU architecture, defaulting to t3.small."
slug: node_type
url: /configuration/rack-parameters/aws/node_type
---

# node_type

## Description
The `node_type` parameter specifies the instance type for the nodes in the cluster. This determines the compute, memory, and network resources allocated to each node. From Rack version `3.25.7` it also accepts a comma separated list of instance types; see [Multiple Instance Types](#multiple-instance-types).

## Default Value
The default value for `node_type` is `t3.small`.

## Use Cases
- **Resource Allocation**: Choose an instance type that matches the resource requirements of your applications.
- **Performance Optimization**: Select instance types that provide the necessary compute power and memory to ensure optimal performance.

## Setting Parameters
To set the `node_type` parameter, use the following command:
```bash
$ convox rack params set node_type=c5.large -r rackName
Updating parameters... OK
```
This command sets the node instance type to `c5.large`.

## Multiple Instance Types

`node_type` accepts a comma separated list of instance types on Rack version `3.25.7` or later. Each per-availability-zone node group is then created with every listed type, so a group can still launch a node when EC2 has no capacity for the first type in that zone. A managed node group takes up to 20 instance types, the EKS limit.

```bash
$ convox rack params set node_type=m5a.2xlarge,m5.2xlarge,m6a.2xlarge -r rackName
Updating parameters... OK
```

Write the list with no spaces. The parameter pattern rejects `m5a.2xlarge, m5.2xlarge`, so the Console refuses a padded value, while `convox rack params set` accepts it and stores it as typed.

An On-Demand node group tries the types in the order listed, so the first entry is what normally launches and a later entry launches only when the earlier ones are short in that zone. Nothing is rebuilt when that happens: one instance launches on a different type, and the next launch tries the first entry again. When [`node_capacity_type`](/configuration/rack-parameters/aws/node_capacity_type) puts a group on Spot, EKS chooses from the list by available capacity and price and ignores the order, so more types there widen the pool rather than set a preference.

On a healthy Rack every group runs the first entry and the list changes nothing you can observe.

A Rack below `3.25.7` accepts the list and uses only the first entry on the primary node groups, with no warning. `convox rack params` reports the stored list on any version, so the parameter value is not evidence that the fallback is active.

### List Instance Types of the Same Size

List types with the same vCPU count and memory, across families if you like. `m5a.2xlarge`, `m5.2xlarge`, `m6a.2xlarge` and `m6i.2xlarge` are all 8 vCPU and 32 GiB.

There are two reasons. The Cluster Autoscaler's scale-up arithmetic assumes the types in a node group are interchangeable in size, and mismatched sizes give it the wrong node count. The cross-zone scale-up split also compares the per-availability-zone groups on CPU count, pod capacity, ephemeral storage and memory, so groups running different sizes do not compare equal and the split stops until the fleet converges. From `3.25.7` the instance type name itself is left out of that comparison, which is why same-size types from different families keep the split.

Listing different sizes is allowed. It widens the capacity pool and stops the cross-zone split. See [How the Autoscaler Picks a Group](/configuration/scaling/autoscaling#how-the-autoscaler-picks-a-group).

### Rejected Combinations

`convox rack params set` refuses these before anything reaches the Rack:

| Value | Error |
|-------|-------|
| `node_type=m5.large,m6g.large` | `node_type mixes CPU architectures: m5.large is amd64, m6g.large is arm64; all entries must share one architecture, since the node AMI is selected from the first entry` |
| `node_type=m5.large,g5.xlarge` | `node_type mixes GPU and non-GPU instance types: m5.large, g5.xlarge; all entries must match, since the node AMI is selected from the first entry` |
| `build_node_type=m5.large,m5a.large` | `build_node_type takes a single instance type; use node_type for a list` |

The node group AMI is selected from the first entry, so EKS refuses a list that spans architectures, and an accelerator instance launched from a non-accelerator AMI has no driver. An entry whose family the CLI cannot classify, such as a hyphenated family like `c7i-flex.large`, is skipped by the architecture check rather than rejected, so a new instance family is never blocked by an older CLI.

These checks run on `convox rack params set`. A Rack updated from the Console does not run them, and the same combination fails later in the Terraform apply with the EKS error. Correcting the parameter recovers either way, and nothing is destroyed.

### Changing the List

Changing `node_type` replaces the primary node groups under new names, new nodes first and then the old ones drain. Any `node_type` change does this, whether the value is one type or a list. Setting the list back to one type replaces them again, and downgrading a Rack that holds a list replaces them once more with the first entry pinned.

One replacement happens with no parameter change. A Rack that already held a comma separated `node_type` before `3.25.7` was running only the first entry, and the upgrade starts using the whole list, which renames the per-availability-zone system node groups and replaces them once during the version update. The build node group name does not change on either version, so that group is left alone. Set `node_type` to a single instance type before the upgrade if you would rather not replace them then.

On a [Karpenter](/configuration/scaling/karpenter) Rack the workload nodes come from `karpenter_instance_families` and `karpenter_instance_sizes`, so the list applies to the system node groups only.

## CPU Architecture (x86 vs ARM)

Convox supports both x86 (Intel/AMD) and ARM (Graviton) instance types. The `node_type` sets the CPU architecture for the Rack's system components, and it is the default architecture for [`build_node_type`](/configuration/rack-parameters/aws/build_node_type) and, on [Karpenter](/configuration/scaling/karpenter) Racks, for [`karpenter_arch`](/configuration/rack-parameters/aws/karpenter_arch) when that parameter is left unset. When `node_type` names a list, the architecture comes from the first entry, and `convox rack params set` rejects a list whose entries do not share one architecture.

**x86 instance families** (default): `t3`, `c5`, `m5`, `r5`, `c6i`, `m6i`, etc.

**ARM/Graviton instance families**: `t4g`, `c6g`, `c7g`, `m6g`, `r6g`, `a1`, etc.

Node groups added with [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config) and [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config) may use a different CPU architecture from `node_type`. Convox selects an arm64 or x86 EKS AMI for each node group from that node group's own instance type, so x86 and ARM node groups can coexist in one Rack.

What does not follow automatically is your App's image. An image built for one architecture will not run on nodes of the other, and Processes scheduled onto a mismatched node fail with an exec format error. On a mixed-architecture Rack, use the [`BuildArch`](/configuration/app-parameters/aws/BuildArch) app parameter to pin each App's image to the right architecture and `nodeSelectorLabels` to keep its Services on matching nodes. See [Workload Placement](/configuration/scaling/workload-placement).

## Additional Information
Selecting the appropriate instance type for your nodes is crucial for achieving the desired performance and cost-efficiency. AWS offers a variety of instance types, each with different combinations of CPU, memory, storage, and networking capacity. Consider your application's specific needs when choosing an instance type. For more information on AWS EC2 instance types, refer to the [AWS documentation on EC2 instance types](https://docs.aws.amazon.com/ec2/latest/instancetypes/ec2-instance-type-specifications.html).
