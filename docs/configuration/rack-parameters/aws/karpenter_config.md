---
title: "karpenter_config"
description: "The karpenter_config AWS rack parameter is a JSON escape hatch for the Karpenter workload NodePool and EC2NodeClass to reach features not exposed as parameters."
slug: karpenter_config
url: /configuration/rack-parameters/aws/karpenter_config
---

# karpenter_config

## Description

The `karpenter_config` parameter provides a JSON escape hatch for the [Karpenter](/configuration/scaling/karpenter) workload NodePool and its EC2NodeClass. Use this to access Karpenter API features not exposed as individual parameters, such as several disruption windows, a per-reason budget cap, or advanced block device mappings.

A single recurring disruption window is [`karpenter_disruption_block_schedule`](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule) and [`karpenter_disruption_block_duration`](/configuration/rack-parameters/aws/karpenter_disruption_block_duration), and pinning the AL2023 AMI version is [`karpenter_ami_alias`](/configuration/rack-parameters/aws/karpenter_ami_alias). Both reach the build and custom pools, which `karpenter_config` does not. Reach for `karpenter_config` for what those parameters cannot express.

Individual `karpenter_*` parameters build the defaults. `karpenter_config` overrides them at the section level. For example, setting `nodePool.template.spec.requirements` in the config completely replaces the defaults built from `karpenter_instance_families`, `karpenter_instance_sizes`, etc.

## Default Value

The default value is empty (no overrides).

## Setting the Parameter

**Using a JSON string:**

```bash
$ convox rack params set karpenter_config='{"nodePool":{"disruption":{"budgets":[{"nodes":"10%"},{"nodes":"0","schedule":"0 9 * * mon-fri","duration":"8h"}]}}}' -r rackName
Updating parameters... OK
```

**Using a JSON file:**

```bash
$ convox rack params set karpenter_config=/path/to/karpenter-config.json -r rackName
Updating parameters... OK
```

## Additional Information

- **Input formats:** Raw JSON string, base64-encoded JSON, or a `.json` file path. Maximum 64KB.
- **Allowed top-level keys:** `nodePool` and `ec2NodeClass` only.
- **Protected fields** that cannot be overridden: `ec2NodeClass.role`, `ec2NodeClass.instanceProfile`, `ec2NodeClass.subnetSelectorTerms`, `ec2NodeClass.securityGroupSelectorTerms`, `nodePool.template.spec.nodeClassRef`, `nodePool.template.metadata.labels["convox.io/nodepool"]`, `ec2NodeClass.tags.Name`, `ec2NodeClass.tags.Rack`.
- **`amiSelectorTerms` replaces the generated selector.** Setting `ec2NodeClass.amiSelectorTerms` for any reason, not only to pin an AMI, takes the workload pool out of [`karpenter_ami_alias`](/configuration/rack-parameters/aws/karpenter_ami_alias), which no longer reaches that pool. The build and additional pools stay pinned.
- **`amiSelectorTerms` and `userData` suppress the kubelet registry settings.** Setting either one suppresses [`kubelet_registry_pull_qps`](/configuration/rack-parameters/aws/kubelet_registry_pull_qps) and [`kubelet_registry_burst`](/configuration/rack-parameters/aws/kubelet_registry_burst) on the workload pool, each on its own, because both replace the generated NodeConfig. The build and additional pools do not read `karpenter_config` and keep those settings.
- See the [Karpenter](/configuration/scaling/karpenter#karpenter_config-workload-nodepool-override) feature page for the full JSON structure, available fields, and examples.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [additional_karpenter_nodepools_config](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) for creating custom NodePools
- [karpenter_disruption_block_schedule](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule) for a recurring no-disruption window on every pool
- [karpenter_disruption_block_duration](/configuration/rack-parameters/aws/karpenter_disruption_block_duration) for how long that window stays open
- [karpenter_ami_alias](/configuration/rack-parameters/aws/karpenter_ami_alias) for pinning the AL2023 AMI version on every pool
