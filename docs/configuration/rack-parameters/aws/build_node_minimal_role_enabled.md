---
title: "build_node_minimal_role_enabled"
description: "The build_node_minimal_role_enabled AWS rack parameter runs build nodes under a dedicated least-privilege IAM role instead of the shared node role."
slug: build_node_minimal_role_enabled
url: /configuration/rack-parameters/aws/build_node_minimal_role_enabled
---

# build_node_minimal_role_enabled

## Description

The `build_node_minimal_role_enabled` parameter gives build nodes a dedicated least-privilege IAM role, `rackName-build-nodes`, instead of the shared node role.

The dedicated role carries only the four AWS managed policies a build node needs: `AmazonEKSWorkerNodePolicy` (cluster join), `AmazonEKS_CNI_Policy` (Pod networking), `AmazonEC2ContainerRegistryReadOnly` (image pulls), and `AmazonSSMManagedInstanceCore` (SSM access). It omits the EBS CSI driver policy that the shared node roles carry and the inline Pod identity policy attached to the classic shared node role. Build nodes are ephemeral, mount no persistent EBS volumes, and Build Processes authenticate to ECR with a build-scoped token rather than the node role, so neither policy is needed.

The parameter applies to the [Karpenter](/configuration/scaling/karpenter) build NodePool (enabled by [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled)) and to any node groups defined in [additional_build_groups_config](/configuration/rack-parameters/aws/additional_build_groups_config). It takes effect only when [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled) is `true` and has no effect on Racks without Karpenter.

## Default Value

The default value is `false`. With the default, build nodes continue to use the shared node role, and a Rack that upgrades without setting the parameter sees no infrastructure change.

## Setting the Parameter

```bash
$ convox rack params set build_node_minimal_role_enabled=true -r rackName
Updating parameters... OK
```

Enabling the parameter applies in a single rack update: the update creates the `rackName-build-nodes` role, registers it with the EKS cluster, and repoints build capacity at it. Build nodes launched by Karpenter from that point run under the dedicated role. Node groups defined in `additional_build_groups_config` are recreated once because their node role changes, so enable or disable during a quiet period to avoid interrupting Builds in flight on those nodes.

Setting the parameter back to `false` returns build nodes to the shared node role and removes the dedicated role automatically, with no manual cleanup required.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.24.10` or later.

- **Validation:** Must be `true` or `false`.
- If a Build depends on AWS permissions granted through the shared node role's instance credentials (for example, running `aws s3 cp` inside a Dockerfile, or relying on policies attached to the shared role outside of Convox), enabling this parameter removes that access. Pass credentials explicitly through build args or App environment variables instead.
- When [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) is configured with Docker Hub credentials, the dedicated role receives the pull-through cache policy, so cached pulls keep working.
- Node groups defined in `additional_build_groups_config` otherwise run under the classic shared node role, which does not include `AmazonSSMManagedInstanceCore`, so enabling this parameter adds SSM access to those nodes in addition to the reductions above.
- The parameter is listed under both the `build` and `security` groups in `convox rack params`.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled)
- [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled)
- [additional_build_groups_config](/configuration/rack-parameters/aws/additional_build_groups_config)
