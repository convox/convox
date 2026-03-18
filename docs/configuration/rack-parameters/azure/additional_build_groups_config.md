---
title: "additional_build_groups_config"
slug: additional_build_groups_config
url: /configuration/rack-parameters/azure/additional_build_groups_config
---

# additional_build_groups_config

## Description
The `additional_build_groups_config` parameter allows you to configure dedicated build node pools for your AKS cluster. Build node pools are automatically tainted with `dedicated=build:NoSchedule` so only build workloads are scheduled on them, providing isolation from your application workloads.

## Default Value
The default value for `additional_build_groups_config` is an empty array.

## Use Cases
- **Build Isolation**: Separate build workloads from application workloads so that builds don't consume application resources.
- **Cost Optimization**: Use differently sized VMs or Spot instances for build workloads.
- **Performance**: Use larger VM sizes for builds to reduce build times without sizing up your entire cluster.

## Configuration Format
The `additional_build_groups_config` parameter takes a JSON array of build node pool configurations:

| Field | Required | Description | Default |
|-------|----------|-------------|---------|
| `type` | Yes | The Azure VM size to use for the build node pool |  |
| `disk` | No | The OS disk size in GB for the nodes | Same as main node disk (default: 30) |
| `capacity_type` | No | Whether to use regular or spot VMs. Use `ON_DEMAND` or `SPOT` (case-insensitive). The aliases `Regular` and `Spot` are also accepted. | `ON_DEMAND` (Regular) |
| `min_size` | No | Minimum number of nodes | 0 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value. Applied as `convox.io/label: <label-value>` | `custom-build` |
| `id` | No | A unique integer identifier for the node pool | Auto-generated |
| `tags` | No | Custom Azure tags as comma-separated key-value pairs | None |
| `zones` | No | Comma-separated list of Azure availability zones | None |

Build node pools automatically get the following labels and taints:
- Label: `convox-build: true`
- Label: `convox.io/label: <label>` (defaults to `custom-build`)
- Taint: `dedicated=build:NoSchedule`

## Setting Parameters

### Using a JSON File (Recommended)
```bash
$ convox rack params set additional_build_groups_config=/path/to/build-config.json -r rackName
Setting parameters... OK
```

Example JSON file:
```json
[
  {
    "id": 201,
    "type": "Standard_D8s_v3",
    "disk": 100,
    "capacity_type": "ON_DEMAND",
    "min_size": 0,
    "max_size": 3,
    "label": "builds"
  }
]
```

### Using a Raw JSON String
```bash
$ convox rack params set 'additional_build_groups_config=[{"id":201,"type":"Standard_D8s_v3","disk":100,"min_size":0,"max_size":3,"label":"builds"}]' -r rackName
Setting parameters... OK
```

## Additional Information
For general-purpose node pools, see the [`additional_node_groups_config`](/configuration/rack-parameters/azure/additional_node_groups_config) parameter.
