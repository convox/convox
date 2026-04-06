---
title: "karpenter_auth_mode"
slug: karpenter_auth_mode
url: /configuration/rack-parameters/aws/karpenter_auth_mode
---

# karpenter_auth_mode

## Description

The `karpenter_auth_mode` parameter is a **one-way migration** that prepares the EKS cluster for [Karpenter](/configuration/scaling/karpenter). When set to `true`, it migrates the EKS cluster to `API_AND_CONFIG_MAP` access mode and applies `karpenter.sh/discovery` tags to subnets and security groups.

This parameter must be `true` before or alongside setting `karpenter_enabled=true`. It can be set in the same call:

```bash
$ convox rack params set karpenter_auth_mode=true karpenter_enabled=true -r rackName
Setting parameters... OK
```

## Default Value

The default value is `false`.

## Use Cases

- **Enabling Karpenter:** Required prerequisite before Karpenter can be deployed. Set it alongside `karpenter_enabled=true` for a single-step enablement.
- **Pre-staging:** Set `karpenter_auth_mode=true` first to apply the prerequisites, then enable Karpenter later with `karpenter_enabled=true` in a separate update.

## Setting the Parameter

```bash
$ convox rack params set karpenter_auth_mode=true -r rackName
Setting parameters... OK
```

## Additional Information

- **One-way migration.** Once set to `true`, this parameter cannot be set back to `false`. The CLI rejects the attempt with an error.
- **Safe to leave enabled.** The access config migration and discovery tags have no cost or operational impact when Karpenter is not active. They simply keep the cluster ready for Karpenter to be enabled at any time.
- **What it changes:**
  - EKS cluster authentication mode set to `API_AND_CONFIG_MAP`
  - `karpenter.sh/discovery` tags applied to public and private subnets
  - `karpenter.sh/discovery` tags applied to the cluster security group
  - System nodes receive the `convox.io/system-node=true` label

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled) for the bidirectional Karpenter toggle
