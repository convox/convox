---
title: "additional_build_groups_config"
description: "The additional_build_groups_config Azure rack parameter configures build node pools for the AKS cluster, tainted to keep application Pods off them."
slug: additional_build_groups_config
url: /configuration/rack-parameters/azure/additional_build_groups_config
---

# additional_build_groups_config

## Description
The `additional_build_groups_config` parameter allows you to configure dedicated build node pools for your AKS cluster. Build node pools are automatically tainted with `dedicated=build:NoSchedule`, which keeps application Pods off them.

Builds tolerate that taint, but nothing directs them to a build pool. A Build runs on a build pool when the App's [`BuildLabels`](/configuration/app-parameters/aws/BuildLabels) app parameter selects the pool's label. Without `BuildLabels`, a Build can run on the default node pool, on an additional node pool that has no taint, or on any build pool.

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
| `capacity_type` | No | Whether to use regular or spot VMs. Accepts `ON_DEMAND`, `SPOT`, `Regular`, or `Spot`, matched exactly. Lowercase forms such as `spot` are rejected | `ON_DEMAND` (Regular) |
| `min_size` | No | Minimum number of nodes | 0 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value. Applied as `convox.io/label: <label-value>` | `custom-build` |
| `id` | No | A unique integer identifier for the node pool | The entry's position in the array |
| `tags` | No | Custom Azure tags as comma-separated key-value pairs | None |
| `zones` | No | Comma-separated list of Azure availability zones. Requires a `convox` CLI at 3.25.3 or newer | None |

> **`zones` requires a `convox` CLI at 3.25.3 or newer.** Earlier CLIs dropped the `zones` key from the configuration before it reached Terraform, so a value set with an older CLI had no effect and no error.

Build node pools automatically get the following labels and taints:
- Label: `convox-build: true`
- Label: `convox.io/label: <label>` (defaults to `custom-build`)
- Taint: `dedicated=build:NoSchedule`

## Setting Parameters

### Using a JSON File (Recommended)
```bash
$ convox rack params set additional_build_groups_config=/path/to/build-config.json -r rackName
Updating parameters... OK
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
Updating parameters... OK
```

### Directing an App's Builds to the Pool
Set `BuildLabels` on each App whose Builds should run on the pool:

```bash
$ convox apps params set BuildLabels=convox.io/label=builds -a myapp
Updating parameters... OK
```

## Arm Build Pools

From Rack version 3.25.12, a build pool can use an Arm VM size, such as `Standard_D4ps_v5`. Build pools have no `dedicated` key, so any App's Build can run on an Arm build pool while the pool has nodes. When a Build of an App without the [`BuildArch`](/configuration/app-parameters/aws/BuildArch) app parameter runs there, it produces an arm64-only image, and that App's Processes then fail on x86 nodes. Set `BuildArch` on every App on the Rack before you add an Arm build pool.

CLI 3.25.12 or later accepts an Arm build pool and prints:

```
WARNING: additional_build_groups_config type Standard_D4ps_v5 is an Arm machine type. Builds of apps without the BuildArch app parameter can run on it and produce Arm-only images.
```

To build one App natively on Arm without exposing other Apps' Builds to Arm nodes, point that App's `BuildLabels` at a dedicated Arm node pool instead; see [Arm Node Pools](/configuration/rack-parameters/azure/additional_node_groups_config#arm-node-pools).

## Additional Information
For general-purpose node pools, see the [`additional_node_groups_config`](/configuration/rack-parameters/azure/additional_node_groups_config) parameter.
