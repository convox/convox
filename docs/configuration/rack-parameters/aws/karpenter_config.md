---
title: "karpenter_config"
slug: karpenter_config
url: /configuration/rack-parameters/aws/karpenter_config
---

# karpenter_config

## Description

The `karpenter_config` parameter provides a JSON escape hatch for the [Karpenter](/configuration/scaling/karpenter) workload NodePool and its EC2NodeClass. Use this to access Karpenter API features not exposed as individual parameters, such as disruption scheduling windows, custom AMI selection, or advanced block device mappings.

Individual `karpenter_*` parameters build the defaults. `karpenter_config` overrides them at the section level — for example, setting `nodePool.template.spec.requirements` in the config completely replaces the defaults built from `karpenter_instance_families`, `karpenter_instance_sizes`, etc.

## Default Value

The default value is empty (no overrides).

## Setting the Parameter

**Using a JSON string:**

```bash
$ convox rack params set karpenter_config='{"nodePool":{"disruption":{"budgets":[{"nodes":"10%"},{"nodes":"0","schedule":"0 9 * * mon-fri","duration":"8h"}]}}}' -r rackName
Setting parameters... OK
```

**Using a JSON file:**

```bash
$ convox rack params set karpenter_config=/path/to/karpenter-config.json -r rackName
Setting parameters... OK
```

## Additional Information

- **Input formats:** Raw JSON string, base64-encoded JSON, or a `.json` file path. Maximum 64KB.
- **Allowed top-level keys:** `nodePool` and `ec2NodeClass` only.
- **Protected fields** that cannot be overridden: `ec2NodeClass.role`, `ec2NodeClass.instanceProfile`, `ec2NodeClass.subnetSelectorTerms`, `ec2NodeClass.securityGroupSelectorTerms`, `nodePool.template.spec.nodeClassRef`, `nodePool.template.metadata.labels["convox.io/nodepool"]`, `ec2NodeClass.tags.Name`, `ec2NodeClass.tags.Rack`.
- See the [Karpenter](/configuration/scaling/karpenter#karpenter_config--workload-nodepool-override) feature page for the full JSON structure, available fields, and examples.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [additional_karpenter_nodepools_config](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) for creating custom NodePools
