---
title: "additional_karpenter_nodepools_config"
slug: additional_karpenter_nodepools_config
url: /configuration/rack-parameters/aws/additional_karpenter_nodepools_config
---

# additional_karpenter_nodepools_config

## Description

The `additional_karpenter_nodepools_config` parameter creates custom [Karpenter](/configuration/scaling/karpenter) NodePools beyond the built-in workload and build pools. Each entry in the JSON array produces its own NodePool + EC2NodeClass pair with the same infrastructure settings (subnet discovery, security groups, IAM role) as the workload pool.

Use this for dedicated GPU pools, tenant isolation, specialized instance requirements, or batch processing pools.

## Default Value

The default value is empty (no custom NodePools).

## Setting the Parameter

**Using a JSON string:**

```bash
$ convox rack params set additional_karpenter_nodepools_config='[{"name":"gpu","instance_families":"g5,g6","capacity_types":"on-demand","cpu_limit":64,"memory_limit_gb":256,"taints":"nvidia.com/gpu=true:NoSchedule","disk":200}]' -r rackName
Setting parameters... OK
```

Target Services to the GPU pool using `nodeSelectorLabels` and `scale.gpu` in `convox.yml`:

```yaml
services:
  ml-worker:
    build: .
    scale:
      gpu:
        count: 1
        vendor: nvidia
    nodeSelectorLabels:
      convox.io/nodepool: gpu
```

**Using a JSON file:**

```bash
$ convox rack params set additional_karpenter_nodepools_config=/path/to/nodepools.json -r rackName
Setting parameters... OK
```

## Additional Information

- **Input formats:** Raw JSON string, base64-encoded JSON, or a `.json` file path.
- Every custom pool automatically gets a `convox.io/nodepool={name}` label. Target Services to a custom pool using `nodeSelectorLabels` in `convox.yml`.
- **Pool name validation:** Lowercase alphanumeric with dashes, max 63 chars. Reserved names: `workload`, `build`, `default`, `system`. Duplicate names are rejected.
- **Pool isolation:** Set `"dedicated": true` on a pool entry to automatically add a `dedicated-node={name}:NoSchedule` taint. Convox auto-injects the matching toleration for Services targeting the pool via `nodeSelectorLabels`. This is the simplest way to isolate a pool without manual taint configuration.
- For pools with custom taints beyond `dedicated`, see [Using Taints to Protect Nodes](/configuration/scaling/karpenter#using-taints-to-protect-nodes) for how tolerations are handled (GPU taints are auto-tolerated via `scale.gpu`; `convox.yml` does not have a `tolerations` field).
- See the [Karpenter](/configuration/scaling/karpenter#additional_karpenter_nodepools_config--custom-nodepools) feature page for the full per-pool field reference and examples.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_config](/configuration/rack-parameters/aws/karpenter_config) for overriding the workload NodePool
- [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config) for custom EKS managed node groups
- [Workload Placement](/configuration/scaling/workload-placement) for node targeting with `nodeSelectorLabels`
