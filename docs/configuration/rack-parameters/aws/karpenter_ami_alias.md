---
title: "karpenter_ami_alias"
description: "The karpenter_ami_alias AWS rack parameter pins the AL2023 Karpenter node pools to one EKS-optimized AMI version instead of tracking the latest published one."
slug: karpenter_ami_alias
url: /configuration/rack-parameters/aws/karpenter_ami_alias
---

# karpenter_ami_alias

## Description

The `karpenter_ami_alias` parameter pins the AL2023 [Karpenter](/configuration/scaling/karpenter) NodePools to one EKS-optimized AMI version. Unset, every pool tracks `al2023@latest` and takes each AL2023 AMI as AWS publishes it. Set, every AL2023 pool launches nodes from the version you name until you change or clear the parameter.

## Default Value

The default value is `""`, which resolves to `al2023@latest`.

`convox rack params` lists stored values only. This parameter does not appear in that output until you set it, so an absent entry means the default is in effect.

## What the Parameter Pins

| NodePool | Behavior |
|----------|----------|
| Workload | Pinned, unless [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config) sets `ec2NodeClass.amiSelectorTerms`, which replaces the whole selector and wins. On a [`karpenter_node_os=bottlerocket`](/configuration/rack-parameters/aws/karpenter_node_os) Rack the workload pool runs `bottlerocket@latest` and this parameter does not reach it |
| Build | Pinned. No earlier version offered any way to pin this pool |
| Additional ([`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config)) | Pinned, unless that pool's `ami_id` is set, which wins |

Setting `ec2NodeClass.amiSelectorTerms` for any reason, not only to pin an AMI, takes the workload pool out of the Rack-wide pin. The build and additional pools stay pinned.

## Setting the Parameter

```bash
$ convox rack params set karpenter_ami_alias=al2023@v20260828 -r rackName
Updating parameters... OK
```

A value in any other shape is rejected before anything is submitted:

```text
karpenter_ami_alias pins AL2023 node pools and must be al2023@latest or al2023@vYYYYMMDD (e.g. al2023@v20260828)
```

Leaving the parameter unset changes nothing. Setting it to the version the fleet already runs replaces no nodes, because `amiSelectorTerms` is excluded from the EC2NodeClass drift hash. Setting it to any other version replaces nodes on every pool, paced by each pool's disruption budget. The replacement runs as drift, so an armed [`karpenter_disruption_block_schedule`](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule) window holds it until the window lifts.

## Clearing the Pin

```bash
$ convox rack params set karpenter_ami_alias= -r rackName
Updating parameters... OK
```

Clearing reverts to the default, so every pool returns to `al2023@latest` on that apply. If AWS has published a newer AMI since the pin was set, nodes are replaced then. The key stays in the `convox rack params` listing with an empty value rather than disappearing.

## Confirming the Version Exists

Karpenter resolves the AMI from an SSM path built from the Kubernetes version, the architecture, the variant and the AMI version together:

```text
/aws/service/eks/optimized-ami/<k8s-minor>/amazon-linux-2023/<arch>/<variant>/amazon-eks-node-al2023-<arch>-<variant>-<k8s-minor>-<version>/image_id
```

`<arch>` is `x86_64` or `arm64`. `<variant>` is `standard`, `nvidia` or `neuron`. A version with no parameter for a pair the Rack can launch leaves that pool unable to launch nodes, and the apply still succeeds. Check every pair the Rack can launch, one lookup each:

```bash
$ aws ssm get-parameter --name /aws/service/eks/optimized-ami/1.35/amazon-linux-2023/x86_64/standard/amazon-eks-node-al2023-x86_64-standard-1.35-v20260828/image_id
```

This path shape is for a dated pin. For `al2023@latest` the final segment is the literal `recommended` rather than an `amazon-eks-node-al2023-` name, so the command above does not verify `latest`. A `--recursive` listing of the family root merges every architecture and variant and proves nothing about any single one.

## Kubernetes Version Upgrades

A pin is not frozen across a Kubernetes upgrade. A Kubernetes version change re-resolves the same AMI version against the new control plane and replaces nodes once. If the pinned version predates the new Kubernetes minor the SSM path does not exist, and every Karpenter pool stops being able to launch a node right after the upgrade. Clear or advance the pin before a Kubernetes version upgrade.

## Finding the Version a Pool Runs

Reading `status.amis` on its own is misleading. Once AWS publishes, that field holds the new AMI while nodes still run the old one, so pinning to the value it reports pins forward and replaces nodes. Match what the class resolves against what a running node actually booted from.

Each pool has its own EC2NodeClass named for the pool: `workload`, `build`, and one per entry in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) named for that entry's `name`. List them:

```bash
$ convox api get /kubernetes/apis/karpenter.k8s.aws/v1/ec2nodeclasses -r rackName | jq -r '.items[].metadata.name'
build
gpu
workload
```

Then take two reads and match them. The first is the class for the pool you are checking, `build` here:

```bash
$ convox api get /kubernetes/apis/karpenter.k8s.aws/v1/ec2nodeclasses/build -r rackName \
    | jq -r '.status.amis[] | "\(.id)\t\(.name)"' > amis.txt
$ convox api get /kubernetes/apis/karpenter.sh/v1/nodeclaims -r rackName \
    | jq -r '.items[].status.imageID' | sort -u | grep -F -f - amis.txt
ami-0123456789abcdef0	amazon-eks-node-al2023-x86_64-standard-1.35-v20260828
```

The version is the trailing segment of the name, `v20260828` here. An empty result means no running node booted from an AMI this class currently resolves, which is what a pool mid-roll or a class pointing at an AMI nothing runs yet looks like. The NodeClaim list covers every pool, so a match proves the AMI is running somewhere on the Rack rather than on this pool specifically; pools sharing the Rack-wide alias resolve the same AMI.

The path begins with a leading slash and carries no `proxy/` segment. `convox api get` cannot carry a query string, so request the path with no parameters appended. The Rack's Kubernetes proxy requires the admin role, and any other role gets a 403 carrying `admin role required for kubernetes api access`.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.6` or later. Setting it also requires a `convox` CLI at `3.25.6` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

- **Validation:** must be `al2023@latest` or `al2023@vYYYYMMDD`, matching the regex `^al2023@(latest|v[0-9]{8})$`. The match is case-sensitive, so `AL2023@Latest` is rejected.
- **Clearable:** setting an empty value returns every pool to `al2023@latest`.
- The parameter applies to AL2023 pools. A Bottlerocket workload pool pins through [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config).
- **Parameter groups:** listed under the `karpenter` group (`convox rack params -g karpenter -r rackName`).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_os](/configuration/rack-parameters/aws/karpenter_node_os) for the OS the workload pool runs
- [karpenter_config](/configuration/rack-parameters/aws/karpenter_config) for overriding `amiSelectorTerms` on the workload pool
- [additional_karpenter_nodepools_config](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) for the per-pool `ami_id` field
- [karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_node_expiry), and [Telling Drift From Expiry](/configuration/rack-parameters/aws/karpenter_node_expiry#telling-drift-from-expiry) for which of the two replaced a set of nodes
- [karpenter_disruption_block_schedule](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule) for a window that holds the roll a new pin starts
- [karpenter_disruption_block_duration](/configuration/rack-parameters/aws/karpenter_disruption_block_duration) for how long that window stays open
- [GPU Nodes and Custom AMIs](/configuration/scaling/gpu-nodes) for building your own AL2023 AMI
