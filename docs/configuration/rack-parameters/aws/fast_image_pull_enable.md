---
title: "fast_image_pull_enable"
description: "The fast_image_pull_enable AWS rack parameter pulls and unpacks image layers in parallel through the SOCI snapshotter on AL2023 nodes."
slug: fast_image_pull_enable
url: /configuration/rack-parameters/aws/fast_image_pull_enable
---

# fast_image_pull_enable

## Description

The `fast_image_pull_enable` parameter turns on the `FastImagePull` feature gate in the Amazon Linux 2023 NodeConfig. The gate swaps the node's snapshotter to SOCI in parallel pull and unpack mode: layers download concurrently and unpack concurrently within an image, in place of one stream at a time.

The gate works on ordinary OCI images. Lazy loading ships disabled, so no SOCI index and no image rebuild are needed.

The improvement scales with image size. AWS reports minimal gain on small images, and its published benchmark uses a roughly 10 GB image on a volume provisioned at 1,000 MiB/s and 16,000 IOPS. AWS labels the feature experimental.

## Default Value

The default value for `fast_image_pull_enable` is `false`.

## Raise Node Volume Throughput First

AWS documents image pulls getting slower with this gate on below 600 MiB/s of root volume throughput, and the Convox default is the gp3 baseline of 125 MiB/s. Raise [`node_volume_throughput`](/configuration/rack-parameters/aws/node_volume_throughput) to 600 or higher before enabling the gate, or in the same call.

Enabling it at the default throughput is rejected by `convox rack params set` before any Terraform apply:

```bash
$ convox rack params set fast_image_pull_enable=true -r rackName
ERROR: fast_image_pull_enable requires at least 600 MiB/s of node volume throughput.
  AWS documents image pulls getting SLOWER below that with this feature on.
  Currently: node_volume_throughput=125
  Raise node_volume_throughput to 600 or higher in the same call, or re-run with --force to proceed
```

Raising throughput in the same call is accepted, because the check reads the incoming value rather than the stored one:

```bash
$ convox rack params set node_volume_throughput=600 fast_image_pull_enable=true -r rackName
Updating parameters... OK
```

`--force`, short form `-f`, prints the same text as a `WARNING:` on stderr and proceeds.

The check runs in both directions: lowering a node volume parameter back under 600 MiB/s while the gate is on is refused the same way. It fires only on a call that carries the gate itself, one of the four node volume parameters, or `additional_karpenter_nodepools_config`, so an unrelated `convox rack params set` and a `convox rack update` on a Rack that already holds the value both run normally.

Only `convox rack params set` runs it. `convox rack install` does not, and a Rack updated from the Console goes through that path, so the gate can be turned on below the floor there with no refusal and no warning. Terraform applies no floor of its own, so read [`node_volume_throughput`](/configuration/rack-parameters/aws/node_volume_throughput) and the Karpenter volume parameters yourself before enabling the gate on a Console-managed Rack.

On a Rack running [Karpenter](/configuration/scaling/karpenter) the floor is checked on the workload pool, the build pool, and every custom pool whose volume type is `gp3`, each against its own effective throughput. Raise [`karpenter_node_volume_throughput`](/configuration/rack-parameters/aws/karpenter_node_volume_throughput), or a pool's own `volume_throughput`, wherever the value inherited from `node_volume_throughput` does not already clear 600.

A Karpenter Rack whose [`karpenter_node_volume_type`](/configuration/rack-parameters/aws/karpenter_node_volume_type) is not `gp3` takes no provisioned throughput at all. That case prints a line to stderr and proceeds rather than refusing:

```text
WARNING: karpenter_node_volume_type=io2 takes no provisioned throughput, so Karpenter nodes stay at that type's own rate with fast_image_pull_enable on.
```

## Setting the Parameter

**Changing the gate rolls every node group and pool whose NodeConfig gains or loses the stanza, once,** so schedule the change in both directions. On a Rack at the default [`kubelet_registry_pull_qps`](/configuration/rack-parameters/aws/kubelet_registry_pull_qps) and [`kubelet_registry_burst`](/configuration/rack-parameters/aws/kubelet_registry_burst), this parameter is the only reason the node launch templates carry a NodeConfig at all, so setting it back to `false` empties that user data and rolls the same nodes a second time.

```bash
$ convox rack params set node_volume_iops=4000 node_volume_throughput=1000 fast_image_pull_enable=true -r rackName
Updating parameters... OK
```

`fast_image_pull_enable=1` is canonicalized to `true` and meets the throughput check exactly as `=true` does.

Downgrading below `3.25.7` rolls the same nodes again. Parameter reconciliation deletes the parameter from the stored values before the apply runs and prints `NOTICE: removing parameters not supported by version <version>: fast_image_pull_enable` on stderr, the older version renders no `featureGates` stanza, and every managed node group and Karpenter pool that carried it rolls once. On a Rack managed through the Console the value stays in the Console's stored parameters and is applied again when you upgrade back to `3.25.7` or later. On a self-managed Rack, set it again after the upgrade.

## Node Scope

The gate reaches the system node group, the build node group, additional node groups, additional build node groups including entries that carry their own `ami_id`, and the Karpenter workload, build, and additional NodePools.

It switches the snapshotter for every image on the node. There is no per-image, per-Service, or per-namespace scoping.

Three configurations do not receive it:

| Configuration | Result |
|---------------|--------|
| [`karpenter_node_os`](/configuration/rack-parameters/aws/karpenter_node_os) set to `bottlerocket` | Convox renders no userData for the Karpenter workload pool, so the gate never arrives there and those nodes do not roll. The managed node groups and the Karpenter build and additional pools on the same Rack do take it. |
| `karpenter_config.ec2NodeClass.userData` or `karpenter_config.ec2NodeClass.amiSelectorTerms` set | Either one on its own replaces the generated NodeConfig on the Karpenter workload pool and drops the gate there. The build and additional pools keep it. See [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config). |
| An AL2023 AMI pinned below `v20250821`, through [`karpenter_ami_alias`](/configuration/rack-parameters/aws/karpenter_ami_alias) or a pool's `ami_id` | The AMI gained the `FastImagePull` gate in `v20250821`. An older release reads the NodeConfig cleanly and ignores the gate, so nodes boot normally and pull images at the pre-feature rate. Advance the pin before enabling the gate. |

## Node Size Minimum

A node with less than 7 GiB of memory or fewer than 4 vCPUs takes the NodeConfig, ignores the gate, and runs plain overlayfs. There is no error and no log line. The default [`node_type`](/configuration/rack-parameters/aws/node_type) of `t3.small` is below the floor, and so is [`build_node_type`](/configuration/rack-parameters/aws/build_node_type) while it inherits that value, so raise `node_type`, and `build_node_type` where it is set, before enabling the gate. On a Karpenter Rack [`karpenter_instance_sizes`](/configuration/rack-parameters/aws/karpenter_instance_sizes) is unset by default and the workload pool takes whatever size Karpenter picks, including instances under the floor; set it to sizes at or above `xlarge`, or narrow [`karpenter_instance_families`](/configuration/rack-parameters/aws/karpenter_instance_families), to keep the gate active on every workload node. The floor is measured against kernel-online memory rather than the EC2 instance specification, so an instance type close to the boundary can fall below it.

| AL2023 release | Memory floor | vCPU floor |
|----------------|--------------|------------|
| `v20250821` through `v20251007` | 8 GiB | 8 |
| `v20251016` and later | 7 GiB | 4 |

A Rack pinned to a release in the earlier window holds the higher floor.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.7` or later. Setting it also requires a `convox` CLI at `3.25.7` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

- **Amazon Linux 2023 nodes only.** The gate is a field of the AL2023 NodeConfig, which is why a Bottlerocket workload pool never sees it.
- **Enabling Karpenter:** on a Rack with [`high_availability`](/configuration/rack-parameters/aws/high_availability) set to `false`, this parameter cannot be set in the same call as `karpenter_enabled=true`. Set it first, wait for the update to finish, then enable Karpenter.
- **Parameter group:** `nodes`. `convox rack params -g nodes -r rackName` lists it next to the other node settings.
- `convox rack params` lists stored values only, so `fast_image_pull_enable` does not appear in that output until you set it.

## See Also

- [node_volume_throughput](/configuration/rack-parameters/aws/node_volume_throughput) for the 600 MiB/s this gate needs on the managed node groups
- [karpenter_node_volume_throughput](/configuration/rack-parameters/aws/karpenter_node_volume_throughput) for the same floor on the Karpenter pools
- [karpenter_node_os](/configuration/rack-parameters/aws/karpenter_node_os) for the Bottlerocket workload pool, which does not receive this gate
- [karpenter_ami_alias](/configuration/rack-parameters/aws/karpenter_ami_alias) for the AMI pin, which ignores this gate below `al2023@v20250821`
- [kubelet_registry_pull_qps](/configuration/rack-parameters/aws/kubelet_registry_pull_qps) and [kubelet_registry_burst](/configuration/rack-parameters/aws/kubelet_registry_burst) for how many pulls may start at once, which is the other cause of a slow pull
