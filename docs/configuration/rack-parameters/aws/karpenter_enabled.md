---
title: "karpenter_enabled"
slug: karpenter_enabled
url: /configuration/rack-parameters/aws/karpenter_enabled
---

# karpenter_enabled

## Description

The `karpenter_enabled` parameter is a **bidirectional toggle** that deploys or removes [Karpenter](/configuration/scaling/karpenter) node autoscaling on your AWS Rack. When enabled, Karpenter manages workload and build node provisioning through NodePools and EC2NodeClasses, replacing Cluster Autoscaler for those tiers.

Requires [`karpenter_auth_mode=true`](/configuration/rack-parameters/aws/karpenter_auth_mode). Both can be set in a single call:

```bash
$ convox rack params set karpenter_auth_mode=true karpenter_enabled=true -r rackName
Setting parameters... OK
```

## Default Value

The default value is `false`.

## Use Cases

- **Faster node scaling:** Karpenter provisions nodes in seconds rather than the multi-minute Cluster Autoscaler feedback loop.
- **Cost optimization:** Karpenter selects the cheapest instance type that satisfies pod requirements, with automatic spot-to-on-demand fallback.
- **Scale-to-zero builds:** Build nodes are provisioned on-demand and removed after builds complete, eliminating idle build node costs.

## Setting the Parameter

```bash
$ convox rack params set karpenter_enabled=true -r rackName
Setting parameters... OK
```

To disable Karpenter:

```bash
$ convox rack params set karpenter_enabled=false -r rackName
Setting parameters... OK
```

## Enablement Validation (3.24.7+)

The CLI validates parameter combinations when enabling Karpenter:

- **`node_capacity_type` must be `on_demand`** when enabling Karpenter. `spot` or `mixed` capacity types can deadlock node replacement.
- **`node_capacity_type` cannot be changed** while Karpenter is active. Disable Karpenter first.
- **Launch template params blocked on non-HA racks** (`gpu_tag_enable`, `imds_http_tokens`, `ebs_volume_encryption_enabled`, `user_data`, `key_pair_name`, and others). Set them in a separate call before enabling Karpenter.

See [Enablement Validation Guards](/configuration/scaling/karpenter#enablement-validation-guards) for the full list and resolution steps.

## Additional Information

- **Bidirectional.** Can be toggled on and off freely. Disabling cleanly removes all Karpenter resources and restores Cluster Autoscaler.
- **Requires `karpenter_auth_mode=true`.** If `karpenter_auth_mode` is not already enabled, include it in the same call.
- **Additional node groups constraint.** All existing [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config) entries must have `dedicated=true` when Karpenter is enabled.
- **What it deploys:** Karpenter controller, workload NodePool + EC2NodeClass, build NodePool (if `build_node_enabled=true`), IAM roles, SQS interruption queue, and EventBridge rules.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_auth_mode](/configuration/rack-parameters/aws/karpenter_auth_mode) for the one-way EKS prerequisite migration
