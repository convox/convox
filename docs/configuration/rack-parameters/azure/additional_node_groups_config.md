---
title: "additional_node_groups_config"
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
- **Resource Efficiency**: Run different workloads on appropriately sized VMs for optimal resource utilization and cost efficiency.

## Configuration Format
The `additional_node_groups_config` parameter takes a JSON array of node pool configurations. Each node pool configuration is a JSON object with the following fields:

| Field | Required | Description | Default |
|-------|----------|-------------|---------|
| `type` | Yes | The Azure VM size to use for the node pool (e.g., `Standard_D4s_v3`) |  |
| `disk` | No | The OS disk size in GB for the nodes | Same as main node disk (default: 30) |
| `capacity_type` | No | Whether to use regular or spot VMs. Accepts `ON_DEMAND`, `SPOT`, `Regular`, or `Spot` | `ON_DEMAND` (Regular) |
| `min_size` | No | Minimum number of nodes | 1 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value for the node pool. Applied as `convox.io/label: <label-value>` | None |
| `id` | No | A unique integer identifier for the node pool that persists across updates | Auto-generated |
| `tags` | No | Custom Azure tags specified as comma-separated key-value pairs (e.g., `environment=production,team=backend`) | None |
| `dedicated` | No | When `true`, only services with matching node pool labels will be scheduled on these nodes (adds a NoSchedule taint) | `false` |
| `zones` | No | Comma-separated list of Azure availability zones (e.g., `1,2,3`) | None (platform default) |

## Setting Parameters
To set the `additional_node_groups_config` parameter, there are several methods:

### Using a JSON File (Recommended)
```bash
$ convox rack params set additional_node_groups_config=/path/to/node-config.json -r rackName
Setting parameters... OK
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
Setting parameters... OK
```

## Node Pool Identification and Tagging

### Using the `id` Field

The `id` field ensures that node pools preserve their identity during configuration updates:

- Each node pool should have a unique integer identifier
- Using the `id` field prevents unnecessary recreation of node pools when making changes to their configuration
- Consistent `id` values help maintain stable infrastructure during updates

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

## Architecture Compatibility

Convox on Azure requires x86-based VM SKUs. ARM-based VM SKUs are not supported. All node pools must use x86 VM SKUs to match the rack's [node_type](/configuration/rack-parameters/azure/node_type).

## Additional Information
When using dedicated node pools (with `dedicated: true`), only services with matching node selector labels will be scheduled on those nodes. This provides strong isolation for workloads with specific requirements.

For build-specific node pools, see the [`additional_build_groups_config`](/configuration/rack-parameters/azure/additional_build_groups_config) parameter.

Properly configured node pools can significantly improve cluster efficiency, resource utilization, and cost optimization while providing the right resource profiles for different workload types.
