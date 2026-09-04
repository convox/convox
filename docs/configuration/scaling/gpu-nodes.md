---
title: "GPU Nodes and Custom AMIs"
description: "Convox ships no NVIDIA driver. The kernel driver comes from the node's AMI, and a Karpenter node pool can pin its own AL2023 AMI with ami_id on AWS."
slug: gpu-nodes
url: /configuration/scaling/gpu-nodes
---

# GPU Nodes and Custom AMIs

Convox ships no NVIDIA driver. The kernel driver lives in the node's AMI, AWS decides which driver version each EKS-optimized accelerated AMI carries, and a Rack that needs a newer driver branch runs its GPU nodes on an AMI you build yourself. Set `ami_id` on a [Karpenter](/configuration/scaling/karpenter) node pool to point that pool at your AMI.

> Custom AMIs on Karpenter node pools are available on **AWS only**.

## Where the GPU Driver Comes From

Three separate layers make a GPU usable by a Service, and Convox supplies only one of them.

| Layer | Supplied by | Role |
|-------|-------------|------|
| Kernel driver | The node's AMI | Drives the physical GPU. |
| Device plugin | Convox, via [`nvidia_device_plugin_enable`](/configuration/rack-parameters/aws/nvidia_device_plugin_enable) | Advertises `nvidia.com/gpu` to Kubernetes so the scheduler can place pods on GPU nodes. |
| CUDA userspace | Your container image | The libraries your process links against. |

Convox installs no driver, no GPU Operator, and no container toolkit. Enabling `nvidia_device_plugin_enable` installs the device plugin chart and nothing else, so the driver version on a GPU node is whatever its AMI was built with. AWS publishes one driver branch per accelerated AMI release, and running a newer branch means building the AMI yourself.

## Setting a Custom AMI on a Node Pool

Set `ami_id` on an entry in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) to run that pool's nodes on your own AMI:

```bash
$ convox rack params set additional_karpenter_nodepools_config='[{"name":"gpu","instance_families":"g5,g6","ami_id":"ami-0123456789abcdef0","disk":200,"dedicated":true}]' nvidia_device_plugin_enable=true -r rackName
Updating parameters... OK
```

Only that pool changes. Every other custom pool, and the workload and build pools, keep the EKS-optimized AMI Convox selects.

The AMI must be AL2023-based. Convox renders `amiFamily: AL2023` on the pool's EC2NodeClass, and Karpenter reads that family to generate the nodeadm-format userData the node uses to join the cluster.

Target Services to the pool with `nodeSelectorLabels` and request GPUs with `scale.gpu`. See [Using Taints to Protect Nodes](/configuration/scaling/karpenter#using-taints-to-protect-nodes) for how Convox injects the GPU toleration.

EKS managed node groups take the same field, and often give the shorter answer for a Rack not running Karpenter. See [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config) and [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config).

### AMI ID Format

`ami_id` must be `ami-` followed by 8 or 17 lowercase hexadecimal digits. `convox rack params set` rejects anything else:

```text
karpenter nodepool 'gpu': invalid ami_id 'ami-XYZ' (must look like ami-0123456789abcdef0)
```

The check covers format alone. Convox does not check that the AMI exists, that it lives in the Rack's region, that the Rack's account can read it, or that its architecture matches the pool's `arch`.

## Building an AMI with a Newer Driver

The AWS build scripts at [`awslabs/amazon-eks-ami`](https://github.com/awslabs/amazon-eks-ami) take the driver version as a build variable. AWS documents the invocation as:

```bash
$ make k8s=1.35 os_distro=al2023 enable_accelerator=nvidia nvidia_driver_major_version=595
```

Set `k8s` to your Rack's Kubernetes version, which `convox rack params -g versions` prints.

AWS documents the 595 driver as incompatible with P3, P3dn, and G6f instances, so it is not a drop-in replacement for the branch AWS ships by default. Check your instance families and the current build flags against [the AWS driver notes](https://github.com/awslabs/amazon-eks-ami/blob/main/doc/usage/g7-ami.md) before building. Both the flags and the compatibility list change on the AWS release schedule.

## What Changes When You Set ami_id

### AMI Updates Stop

Without `ami_id` the pool follows the Rack-wide alias, which is [`karpenter_ami_alias`](/configuration/rack-parameters/aws/karpenter_ami_alias) where it is set and `al2023@latest` otherwise. On `al2023@latest` Karpenter rolls the pool's nodes onto each new AWS AMI as it is published. Pinning an id takes the pool out of the alias and ends AMI drift for it, including OS security patches. The pool stays on the AMI you named until you name another one.

To move the pool onto a rebuilt AMI, set the new `ami_id`. Karpenter marks the existing nodes drifted and replaces them, bounded by the pool's `disruption_budget_nodes`.

### The AMI Does Not Follow k8s_version

A Rack upgrade to a new Kubernetes minor leaves the pinned AMI where it is. Rebuild the AMI for the new minor and set the new id in the same maintenance window.

### disk Must Fit the AMI

Per AWS, EC2 rejects a launch whose root volume is smaller than the AMI's root snapshot. Read the snapshot size and set the pool's `disk` above it:

```bash
$ aws ec2 describe-images --image-ids ami-0123456789abcdef0 --query 'Images[0].BlockDeviceMappings'
```

## Troubleshooting

A pool can accept an `ami_id` and still not run on the AMI you set. Four causes, none of which produce an error at `convox rack params set`:

| Cause | Result | Fix |
|-------|--------|-----|
| The AMI id is well-formed but does not exist, lives in another region, or is not shared with the Rack's account | No node is provisioned. The EC2NodeClass reports `AMIsReady` as `False` | Correct the id, or copy the AMI into the Rack's region and share it with the Rack's account |
| The pool's `arch` does not match the AMI's architecture | No node is provisioned | Set the pool's `arch` to match the AMI. It defaults to `amd64` |
| The pool's `disk` is smaller than the AMI's root snapshot | EC2 rejects each launch and Karpenter retries | Raise `disk` above the snapshot size |
| The CLI that last edited the pool list predates `3.25.5` | The pool provisions on the AWS AMI, because `ami_id` was dropped from the parameter | Upgrade the CLI with `sudo convox update`, then set `ami_id` again |

Read the pool's EC2NodeClass back to see the AMI condition:

```bash
$ convox api get /kubernetes/apis/karpenter.k8s.aws/v1/ec2nodeclasses/gpu -r rackName
```

Karpenter sets `AMIsReady` to `False` when the selector matches no AMI.

## Version Requirements

The CLI floor and the Rack floor are separate mechanisms. A Rack can satisfy one and not the other.

| Component | Minimum | Below the floor |
|-----------|---------|-----------------|
| `convox` CLI | `3.25.5` | The next edit of the pool list drops `ami_id` |
| Rack | `3.25.5` | The parameter is accepted and the field ignored; the pool keeps the AWS AMI |

`convox rack params set` parses the pool list into a typed structure and writes back what it produces, so a CLI that does not know `ami_id` removes it. On the Rack side the Terraform module has no `ami_id` key to read. Neither case returns an error.

Downgrading a Rack below `3.25.5` renders the pool with the `al2023@latest` alias again and rolls its nodes back onto the AWS AMI, which on a GPU pool means returning to the driver that AMI ships. Upgrading again restores the pinned AMI.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [additional_karpenter_nodepools_config](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) for the per-pool field reference
- [nvidia_device_plugin_enable](/configuration/rack-parameters/aws/nvidia_device_plugin_enable) for advertising node GPUs to Kubernetes
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable) for GPU utilization, VRAM, temperature, and power metrics
- [karpenter_ami_alias](/configuration/rack-parameters/aws/karpenter_ami_alias) for pinning every AL2023 pool to one EKS-optimized AMI version, the lighter alternative to a per-pool `ami_id` because Convox keeps resolving the AMI
- [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config) for custom AMIs on EKS managed node groups
- [additional_build_groups_config](/configuration/rack-parameters/aws/additional_build_groups_config) for custom AMIs on managed build node groups
- [Autoscaling](/configuration/scaling/autoscaling#gpu-scaling) for `scale.gpu` and GPU-based autoscaling
