---
title: "karpenter_node_os"
description: "The karpenter_node_os AWS rack parameter selects the node operating system for Karpenter workload nodes, al2023 (default) or bottlerocket."
slug: karpenter_node_os
url: /configuration/rack-parameters/aws/karpenter_node_os
---

# karpenter_node_os

## Description

The `karpenter_node_os` parameter selects the node operating system for [Karpenter](/configuration/scaling/karpenter)-provisioned workload nodes.

- `al2023`: Amazon Linux 2023 (default).
- `bottlerocket`: The EKS-optimized Bottlerocket AMI, a minimal, container-focused, hardened node OS (immutable root filesystem, enforcing SELinux, no shell or SSH, signed atomic updates).

When set to `bottlerocket`, the workload `EC2NodeClass` selects the Bottlerocket AMI and provisions the two volumes Bottlerocket requires: a 4 GiB `gp3` OS volume on `/dev/xvda` and a data volume on `/dev/xvdb` (sized by [karpenter_node_disk](/configuration/rack-parameters/aws/karpenter_node_disk), falling back to [node_disk](/configuration/rack-parameters/aws/node_disk)) that holds container images, logs, and ephemeral storage. The data volume uses [karpenter_node_volume_type](/configuration/rack-parameters/aws/karpenter_node_volume_type); the OS volume is always `gp3`. Both volumes follow [ebs_volume_encryption_enabled](/configuration/rack-parameters/aws/ebs_volume_encryption_enabled).

This parameter applies only to the workload NodePool and only when [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled) is `true`. System nodes, build nodes, additional node groups, and additional Karpenter NodePools remain on Amazon Linux 2023.

## Default Value

The default value is `al2023`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_os=bottlerocket -r rackName
Updating parameters... OK
```

Changing this value re-renders the workload `EC2NodeClass`. Karpenter detects the AMI drift and gracefully replaces workload nodes, respecting PodDisruptionBudgets. Switching back to `al2023` rolls the nodes back.

## Additional Information

- **Validation:** Must be `al2023` or `bottlerocket`. Values are case-insensitive; the CLI normalizes them to lowercase.
- **Overrides:** `amiSelectorTerms` and `blockDeviceMappings` set through [karpenter_config](/configuration/rack-parameters/aws/karpenter_config) take precedence over the AMI and volume defaults this parameter renders.
- **Operations on Bottlerocket:** there is no SSH or host shell. Host-level access uses AWS Systems Manager Session Manager and the Bottlerocket `apiclient`. The Rack attaches the `AmazonSSMManagedInstanceCore` policy to the Karpenter node role, so no additional IAM configuration is required.
- **The AMI pin does not reach a Bottlerocket workload pool.** With `bottlerocket` the workload `EC2NodeClass` selects `bottlerocket@latest` and [karpenter_ami_alias](/configuration/rack-parameters/aws/karpenter_ami_alias) no longer applies to it; pin that pool through `amiSelectorTerms` in [karpenter_config](/configuration/rack-parameters/aws/karpenter_config) instead. The build and additional pools stay on Amazon Linux 2023 and keep following `karpenter_ami_alias`.
- **GPU workloads** should stay on `al2023`; the Bottlerocket NVIDIA variant is not yet supported.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_node_disk](/configuration/rack-parameters/aws/karpenter_node_disk)
- [karpenter_node_volume_type](/configuration/rack-parameters/aws/karpenter_node_volume_type)
- [karpenter_ami_alias](/configuration/rack-parameters/aws/karpenter_ami_alias) for the AL2023 AMI version pin, which a Bottlerocket workload pool does not follow
