---
title: "additional_build_groups_config"
draft: false
slug: additional_build_groups_config
url: /configuration/rack-parameters/aws/additional_build_groups_config
---

# additional_build_groups_config

## Description
The `additional_build_groups_config` parameter allows you to define dedicated node groups specifically for application build processes. This enables you to isolate build workloads from your production services, optimize resources for build-intensive operations, and configure build-specific infrastructure settings.

This feature works in conjunction with the [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config) parameter, offering a comprehensive approach to infrastructure customization within your Convox rack.

## Default Value
The default value for `additional_build_groups_config` is an empty array.

## Use Cases
- **Build Isolation**: Separate build processes from production workloads to prevent resource contention.
- **Cost Optimization**: Use spot instances for builds to reduce costs for these typically ephemeral workloads.
- **Performance Tuning**: Configure build nodes with higher CPU, memory, or disk resources to speed up build processes.
- **Resource Efficiency**: Ensure build processes don't compete with services for resources during peak usage times.

## Configuration Format
The `additional_build_groups_config` parameter takes a JSON array of node group configurations. Each node group configuration is a JSON object with the following fields:

| Field | Required | Description | Default |
|-------|----------|-------------|---------|
| `type` | Yes (or set the `cpu` and `mem` fields) | The EC2 instance type to use for the build node group |  |
| `cpu` | Yes (or set the `type` field) | The minimum number of vCPUs required from the build node |  |
| `mem` | Yes (or set the `type` field) | The minimum mib of memory required from the build node |  |
| `types` | No (but can use in conjunection with the `cpu` and `mem` fields) | List of instance types to apply your specified `cpu` and `mem` attributes against. All other instance types are ignored, even if they match your specified attributes. You can use strings with one or more wild cards, represented by an asterisk (*), to allow an instance type, size, or generation. |  |
| `disk` | No | The disk size in GB for the nodes | Same as main node disk |
| `capacity_type` | No | Whether to use on-demand or spot instances | `ON_DEMAND` |
| `min_size` | No | Minimum number of nodes | 0 |
| `desired_size` | No | Desired number of nodes | 0 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value for the node group. Applied as `convox.io/label: <label-value>` | None |
| `ami_id`* | No | Custom AMI ID to use | EKS-optimized AMI |

\* **Important**: Custom AMI configuration should be used with extreme caution. AMIs in EKS clusters have strict compatibility requirements, and improper configuration can lead to cluster update failures requiring manual intervention. Only use custom AMIs if you have specific compatibility requirements and thoroughly understand EKS node bootstrapping processes. We recommend testing in a non-production environment before implementation.

## Setting Parameters
To set the `additional_build_groups_config` parameter, there are several methods:

### Using a JSON File (Recommended)
```html
$ convox rack params set additional_build_groups_config=/path/to/build-config.json -r rackName
Setting parameters... OK
```

The JSON file should be structured as follows:
```json
[
  {
    "type": "c5.2xlarge",
    "disk": 100,
    "capacity_type": "SPOT",
    "min_size": 0,
    "desired_size": 1,
    "max_size": 5,
    "label": "app-build"
  }
]
```

> **Important Note on AWS Rate Limits**: When adding or removing multiple node groups, it's recommended to modify no more than three node groups at a time to avoid hitting AWS API rate limits. If you receive a rate limit error during an update simply run the parameter set command again. The operation will resume from where it left off, creating the remaining node groups without duplicating the ones that were already successfully created.

### Using a Raw JSON String
```html
$ convox rack params set 'additional_build_groups_config=[{"type":"c5.2xlarge","disk":100,"capacity_type":"SPOT","min_size":0,"desired_size":1,"max_size":5,"label":"app-build"}]' -r rackName
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

## Directing Build Pods to Specific Node Groups
To direct build pods to specific node groups, use the `BuildLabels` app parameter:

```html
$ convox apps params set BuildLabels=convox.io/label=app-build -a <app>
```

This ensures that build processes for the specified app will run on nodes with the `convox.io/label: app-build` label.

## Customizing Build Pod Resources
You can also specify resource requirements for build pods:

```html
$ convox apps params set BuildCpu=256 BuildMem=1024 -a <app>
```

This sets the CPU request to 256 millicores (0.25 vCPU) and memory request to 1024MB (1GB) for build pods.

## Additional Information
Combining the `additional_build_groups_config` parameter with app-specific `BuildLabels` configuration provides:

1. **Isolation**: Build processes won't interfere with production workloads.
2. **Cost Efficiency**: You can use spot instances for build processes, which are typically tolerant of interruptions.
3. **Resource Optimization**: Custom instance types can be selected based on build requirements.
4. **Scaling Flexibility**: Build node groups can scale based on demand, potentially scaling to zero when no builds are running.

Build nodes configured with larger instance types, more memory, or faster disk I/O can significantly improve build performance for large applications, potentially reducing build times and improving developer productivity.
