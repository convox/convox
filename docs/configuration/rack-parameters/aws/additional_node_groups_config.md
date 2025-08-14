---
title: "additional_node_groups_config"
draft: false
slug: additional_node_groups_config
url: /configuration/rack-parameters/aws/additional_node_groups_config
---

# additional_node_groups_config

## Description
The `additional_node_groups_config` parameter allows you to configure additional customized node groups for your cluster. This feature enables more granular control over your Kubernetes infrastructure by letting you define node groups with specific instance types, capacity types, scaling parameters, and custom labels.

When combined with the `additional_build_groups_config` parameter and node selector configurations, you can optimize workload placement, improve cost efficiency, and separate operational concerns within your cluster.

## Default Value
The default value for `additional_node_groups_config` is an empty array.

## Use Cases
- **Workload-Specific Optimization**: Create node groups tailored to specific workload requirements (e.g., CPU-intensive, memory-intensive, or batch processing workloads).
- **Cost Optimization**: Configure certain node groups to use spot instances for non-critical workloads while maintaining on-demand instances for mission-critical services.
- **Isolation**: Segregate workloads by dedicating specific node groups to particular services.
- **Resource Efficiency**: Run different workloads on appropriately sized instances for optimal resource utilization and cost efficiency.

## Configuration Format
The `additional_node_groups_config` parameter takes a JSON array of node group configurations. Each node group configuration is a JSON object with the following fields:

| Field | Required | Description | Default |
|-------|----------|-------------|---------|
| `type` | Yes (or set the `cpu` and `mem` fields) | The EC2 instance type to use for the node group |  |
| `cpu` | Yes (or set the `type` field) | The minimum number of vCPUs required from the node |  |
| `mem` | Yes (or set the `type` field) | The minimum mib of memory required from the node |  |
| `types` | No (but can use in conjunection with the `cpu` and `mem` fields) | List of instance types to apply your specified `cpu` and `mem` attributes against. All other instance types are ignored, even if they match your specified attributes. You can use strings with one or more wild cards, represented by an asterisk (*), to allow an instance type, size, or generation. |  |
| `disk` | No | The disk size in GB for the nodes | Same as main node disk |
| `capacity_type` | No | Whether to use on-demand or spot instances | `ON_DEMAND` |
| `min_size` | No | Minimum number of nodes | 1 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value for the node group. Applied as `convox.io/label: <label-value>` | None |
| `id` | No | A unique integer identifier for the node group that persists across updates | Auto-generated |
| `tags` | No | Custom AWS tags specified as comma-separated key-value pairs (e.g., `environment=production,team=backend`) | None |
| `ami_id`* | No | Custom AMI ID to use | EKS-optimized AMI |
| `dedicated` | No | When `true`, only services with matching node group labels will be scheduled on these nodes | `false` |

\* **Important**: Custom AMI configuration should be used with extreme caution. AMIs in EKS clusters have strict compatibility requirements, and improper configuration can lead to cluster update failures requiring manual intervention. Only use custom AMIs if you have specific compatibility requirements and thoroughly understand EKS node bootstrapping processes. We recommend testing in a non-production environment before implementation.

## Setting Parameters
To set the `additional_node_groups_config` parameter, there are several methods:

### Using a JSON File (Recommended)
```html
$ convox rack params set additional_node_groups_config=/path/to/node-config.json -r rackName
Setting parameters... OK
```

The JSON file should be structured as follows:
```json
[
  {
    "id": 101,
    "type": "t3.medium",
    "disk": 50,
    "capacity_type": "ON_DEMAND",
    "min_size": 1,
    "max_size": 3,
    "label": "app-workers",
    "tags": "environment=production,team=backend"
  },
  {
    "id": 102,
    "type": "m5.large",
    "disk": 100,
    "capacity_type": "SPOT",
    "min_size": 2,
    "max_size": 5,
    "label": "batch-workers",
    "ami_id": "ami-0123456789abcdef0",
    "tags": "environment=production,team=data,workload=batch"
  }
]
```

> **Important Note on AWS Rate Limits**: When adding or removing multiple node groups, it's recommended to modify no more than three node groups at a time to avoid hitting AWS API rate limits. If you receive a rate limit error during an update simply run the parameter set command again. The operation will resume from where it left off, creating the remaining node groups without duplicating the ones that were already successfully created.

### Using a Raw JSON String
```html
$ convox rack params set 'additional_node_groups_config=[{"id":101,"type":"t3.medium","disk":50,"capacity_type":"ON_DEMAND","min_size":1,"max_size":3,"label":"app-workers","tags":"environment=production,team=backend"}]' -r rackName
Setting parameters... OK
```

## Using `cpu`, `mem` and `types` to allow for more flexibility

To allow for more instance type choice within one node group, instead of specifying a single type, you are able to specify minimum `cpu` and `mem` requirements, and optionally provide a list of instance types to match those against.

```json
[
  {
    "id": 101,
    "cpu": 4,
    "mem": 8192,
    "types": "c3.xlarge, c4.xlarge, c5.xlarge, c5d.xlarge, c5a.xlarge, c5n.xlarge",
    "disk": 50,
    "capacity_type": "ON_DEMAND",
    "min_size": 1,
    "max_size": 3,
    "label": "compute-xlarge"
  },
  {
    "id": 102,
    "cpu": 16,
    "mem": 16384,
    "types": "*.4xlarge, *.8xlarge",
    "disk": 100,
    "capacity_type": "SPOT",
    "min_size": 2,
    "max_size": 5,
    "label": "4xlarge"
  }
]
```

Our first node group (`101`) would use any instance that has at least 4 vCPU and 8GiB of memory that also matches one of the specific types listed.  Our second node group (`102`) would use any instance that has at least 16 vCPU, 16GiB of memory and can be from any family of instance as long as it is a 4xlarge size or an 8xlarge size.
The following are further examples: `m5.8xlarge, c5*.*, m5a.*, r*, *3*.` For example, if you specify `c5*`, you are allowing the entire C5 instance family, which includes all C5a and C5n instance types. If you specify `m5a.*`, you are allowing all the M5a instance types, but not the M5n instance types.

## Node Group Identification and Tagging

### Using the `id` Field

The `id` field ensures that node groups preserve their identity during configuration updates:

- Each node group should have a unique integer identifier
- Using the `id` field prevents unnecessary recreation of node groups when making changes to their configuration
- Without an `id`, Convox generates a random identifier that changes when configurations are updated, potentially causing disruptive node group replacements
- Consistent `id` values help maintain stable infrastructure during updates

Example configuration using the `id` field:
```json
[
  {
    "id": 101,
    "type": "t3.medium",
    "label": "web-services",
    "min_size": 1,
    "max_size": 5
  }
]
```

### Using the `tags` Field

The `tags` field allows you to add AWS tags to specific node groups:

- Tags help with cost allocation, resource organization, and compliance tracking
- Specify tags as comma-separated key-value pairs (e.g., `"environment=production,team=backend"`)
- Tags are applied directly to the AWS node group resources
- Custom tags can be used with AWS cost management and reporting tools

Example configuration using the `tags` field:
```json
[
  {
    "id": 101,
    "type": "t3.medium",
    "label": "web-services",
    "min_size": 1,
    "max_size": 5,
    "tags": "environment=production,team=frontend,tier=web"
  }
]
```

## Using Node Groups with Services
To target specific services to run on particular node groups, use the `nodeSelectorLabels` field in your `convox.yml` file:

```yaml
services:
  web:
    nodeSelectorLabels:
      convox.io/label: app-workers
```

This will ensure that the `web` service is scheduled only on nodes with the label `convox.io/label: app-workers`.

## Additional Information
When using dedicated node groups (with `dedicated: true`), only services with matching node selector labels will be scheduled on those nodes. This provides strong isolation for workloads with specific requirements.

For build-specific node groups, see the [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config) parameter.

Properly configured node groups can significantly improve cluster efficiency, resource utilization, and cost optimization while providing the right resource profiles for different workload types.
