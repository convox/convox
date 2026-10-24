---
title: "additional_node_groups_config"
description: "The additional_node_groups_config Azure rack parameter defines additional AKS node pools with specific VM sizes, capacity types, scaling, and custom labels."
slug: additional_node_groups_config
url: /configuration/rack-parameters/azure/additional_node_groups_config
---

# additional_node_groups_config

## Description
The `additional_node_groups_config` parameter allows you to configure additional customized node pools for your AKS cluster. This feature enables more granular control over your Kubernetes infrastructure by letting you define node pools with specific VM sizes, capacity types, scaling parameters, and custom labels.

When combined with the `additional_build_groups_config` parameter and node selector configurations, you can optimize workload placement, improve cost efficiency, and separate operational concerns within your cluster.

## Default Value
The default value for `additional_node_groups_config` is an empty array.

## Use Cases
- **Workload-Specific Optimization**: Create node pools tailored to specific workload requirements (e.g., CPU-intensive, memory-intensive, GPU, or batch processing workloads).
- **Cost Optimization**: Configure certain node pools to use Spot VMs for non-critical workloads while maintaining regular VMs for mission-critical services.
- **Isolation**: Segregate workloads by dedicating specific node pools to particular services.
- **Arm Workloads**: Run Services on Arm VM sizes next to the x86 default node pool. See [Arm Node Pools](#arm-node-pools).
- **Resource Efficiency**: Run different workloads on appropriately sized VMs for optimal resource utilization and cost efficiency.

## Configuration Format
The `additional_node_groups_config` parameter takes a JSON array of node pool configurations. Each node pool configuration is a JSON object with the following fields:

| Field | Required | Description | Default |
|-------|----------|-------------|---------|
| `type` | Yes | The Azure VM size to use for the node pool (e.g., `Standard_D4s_v3`) |  |
| `disk` | No | The OS disk size in GB for the nodes | Same as main node disk (default: 30) |
| `capacity_type` | No | Whether to use regular or spot VMs. Accepts `ON_DEMAND`, `SPOT`, `Regular`, or `Spot`, matched exactly. Lowercase forms such as `spot` are rejected | `ON_DEMAND` (Regular) |
| `min_size` | No | Minimum number of nodes | 1 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value for the node pool. Applied as `convox.io/label: <label-value>` | None |
| `id` | No | A unique integer identifier for the node pool that persists across updates | The entry's position in the array |
| `tags` | No | Custom Azure tags specified as comma-separated key-value pairs (e.g., `environment=production,team=backend`) | None |
| `dedicated` | No | When `true`, only services with matching node pool labels will be scheduled on these nodes (adds a NoSchedule taint) | `false` |
| `zones` | No | Comma-separated list of Azure availability zones (e.g., `1,2,3`). Requires a `convox` CLI at 3.25.3 or newer | None (platform default) |

> **`zones` requires a `convox` CLI at 3.25.3 or newer.** Earlier CLIs dropped the `zones` key from the configuration before it reached Terraform, so a value set with an older CLI had no effect and no error. The Azure Terraform module itself is unchanged in this release and has consumed `zones` since 3.23.4; the fix is entirely in the CLI.

## Setting Parameters
To set the `additional_node_groups_config` parameter, there are several methods:

### Using a JSON File (Recommended)
```bash
$ convox rack params set additional_node_groups_config=/path/to/node-config.json -r rackName
Updating parameters... OK
```

The JSON file should be structured as follows:
```json
[
  {
    "id": 101,
    "type": "Standard_D4s_v3",
    "disk": 50,
    "capacity_type": "ON_DEMAND",
    "min_size": 1,
    "max_size": 3,
    "label": "app-workers",
    "tags": "environment=production,team=backend"
  },
  {
    "id": 102,
    "type": "Standard_E4s_v3",
    "disk": 100,
    "capacity_type": "SPOT",
    "min_size": 2,
    "max_size": 5,
    "label": "batch-workers",
    "tags": "environment=production,team=data,workload=batch"
  }
]
```

### Using a Raw JSON String
```bash
$ convox rack params set 'additional_node_groups_config=[{"id":101,"type":"Standard_D4s_v3","disk":50,"capacity_type":"ON_DEMAND","min_size":1,"max_size":3,"label":"app-workers","tags":"environment=production,team=backend"}]' -r rackName
Updating parameters... OK
```

## Node Pool Identification and Tagging

### Using the `id` Field

The `id` field keeps a node pool's identity stable across configuration updates:

- Each `id` must be unique. The CLI rejects a configuration with duplicate `id` values, and rejects one where some entries carry an `id` and others do not
- The `id` fixes the pool's identity, so editing other fields or reordering the array does not churn unrelated pools
- It does not prevent recreation. When you change a field AKS treats as immutable, such as `type`, `disk`, or `zones`, the pool rotates create-first through a temporary pool: capacity is preserved, but the nodes are replaced
- Without an `id`, the pool is keyed by its position in the array, so reordering or removing an entry shifts the keys and can apply one pool's configuration to another. Each affected pool rotates through a temporary pool rather than leaving a capacity gap, but its nodes are replaced

Example configuration using the `id` field:
```json
[
  {
    "id": 101,
    "type": "Standard_D4s_v3",
    "label": "web-services",
    "min_size": 1,
    "max_size": 5
  }
]
```

### Using the `tags` Field

The `tags` field allows you to add Azure tags to specific node pools:

- Tags help with cost allocation, resource organization, and compliance tracking
- Specify tags as comma-separated key-value pairs (e.g., `"environment=production,team=backend"`)
- Tags are applied directly to the Azure node pool resources

Example configuration using the `tags` field:
```json
[
  {
    "id": 101,
    "type": "Standard_D4s_v3",
    "label": "web-services",
    "min_size": 1,
    "max_size": 5,
    "tags": "environment=production,team=frontend,tier=web"
  }
]
```

## Changing Zones on an Existing Pool

Azure rotates a node pool when you change its `zones`. It stands up a temporary pool, migrates the workloads onto it, rebuilds the original pool with the new zones, then migrates the workloads back and removes the temporary pool. The pool's capacity is never entirely absent, but workloads on it drain and reschedule twice, so plan the change like any other rolling replacement.

Upgrading an existing Azure Rack to 3.25.3 causes no node pool churn. The Azure Terraform module is unchanged in this release, and a stored `additional_node_groups_config` written by an older CLI contains no `zones` key, so Terraform plans no change to the pools. Zones are applied the first time you set them with a 3.25.3 or newer CLI.

## Spot VM Considerations

When using `capacity_type: "SPOT"` (or `"Spot"`):

- Azure Spot VMs can be evicted at any time when Azure needs the capacity back
- Nodes will automatically be tainted with `kubernetes.azure.com/scalesetpriority=spot:NoSchedule`
- Spot VMs are best suited for fault-tolerant, stateless workloads
- The `spot_max_price` is set to `-1` (pay up to on-demand price) by default

## Using Node Pools with Services
To target specific services to run on particular node pools, use the `nodeSelectorLabels` field in your `convox.yml` file:

```yaml
services:
  web:
    nodeSelectorLabels:
      convox.io/label: app-workers
```

This will ensure that the `web` service is scheduled only on nodes with the label `convox.io/label: app-workers`.

## Arm Node Pools

Arm node pools run next to the x86 default node pool on Rack version 3.25.12 or later. [`node_type`](/configuration/rack-parameters/azure/node_type#cpu-architecture) must stay an x86 VM size, and Arm capacity is added here. The CLI treats a VM size with a `p` after the vCPU count as Arm, such as `Standard_D4ps_v5`, `Standard_E4pds_v5` or `Standard_B2pts_v2`.

Arm VM families have their own vCPU quota in each region, and it is often 0. Request quota for the family in the Rack's region before you add the pool.

Give each Arm pool `dedicated: true` and a `label`:

```json
[
  {
    "id": 202,
    "type": "Standard_D4ps_v5",
    "label": "arm",
    "dedicated": true,
    "min_size": 1,
    "max_size": 2
  }
]
```

```bash
$ convox rack params set additional_node_groups_config=/path/to/arm-pools.json -r rackName
Updating parameters... OK
```

The pool's nodes carry the label `convox.io/label=arm` and the taint `dedicated-node=arm:NoSchedule`, so Pods that do not select the label stay off it. Without `dedicated: true`, CLI 3.25.12 or later accepts the entry and prints:

```
WARNING: additional_node_groups_config type Standard_D4ps_v5 is an Arm machine type without dedicated: true. Pods with x86-only images can schedule on it and fail to start.
```

Pods with no node selector can then schedule on the Arm nodes, and a Pod whose image has no arm64 variant fails to start with `exec format error`. Three of the default containerized Resource images have no arm64 variant; see [Resource](/reference/primitives/app/resource#containerized).

A Service on the pool selects its label:

```yaml
services:
  web:
    build: .
    port: 3000
    nodeSelectorLabels:
      convox.io/label: arm
```

Set [`BuildArch`](/configuration/app-parameters/aws/BuildArch) so the App's image is built for arm64:

```bash
$ convox apps params set BuildArch=arm64 -a myapp
Updating parameters... OK
```

A Build that lands on an x86 node runs under emulation. To build natively on the Arm pool, also point [`BuildLabels`](/configuration/app-parameters/aws/BuildLabels) at it:

```bash
$ convox apps params set BuildLabels=convox.io/label=arm BuildArch=arm64 -a myapp
Updating parameters... OK
```

[`convox builds import-image`](/reference/cli/builds-import-image) copies the `linux/arm64` image from a multi-architecture source for an App with `BuildArch=arm64`. For Arm build pools, see [`additional_build_groups_config`](/configuration/rack-parameters/azure/additional_build_groups_config#arm-build-pools).

## Additional Information
When using dedicated node pools (with `dedicated: true`), only services with matching node selector labels will be scheduled on those nodes. This provides strong isolation for workloads with specific requirements.

For build-specific node pools, see the [`additional_build_groups_config`](/configuration/rack-parameters/azure/additional_build_groups_config) parameter.

Properly configured node pools can significantly improve cluster efficiency, resource utilization, and cost optimization while providing the right resource profiles for different workload types.
