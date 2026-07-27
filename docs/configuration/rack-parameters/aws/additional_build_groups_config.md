---
title: "additional_build_groups_config"
description: "The additional_build_groups_config AWS rack parameter defines dedicated node groups for application build processes, isolating builds from production workloads."
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
| `type` | Yes | The EC2 instance type to use for the build node group |  |
| `disk` | No | The disk size in GB for the nodes | Same as main node disk |
| `capacity_type` | No | Whether to use on-demand or spot instances. Accepts `ON_DEMAND` or `SPOT` only, matched exactly. The Azure aliases `Regular` and `Spot` are rejected | `ON_DEMAND` |
| `min_size` | No | Minimum number of nodes | 0 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value for the node group. Applied as `convox.io/label: <label-value>` | `custom-build` (nodes are labelled `convox.io/label: custom-build` when no label is set) |
| `ami_id`* | No | Custom AMI ID to use | EKS-optimized AMI |

A build group's desired size always tracks `min_size`. There is no separate `desired_size` field, and a key by that name is discarded before the update is submitted.

\* **Important**: Custom AMI configuration should be used with extreme caution. AMIs in EKS clusters have strict compatibility requirements, and improper configuration can lead to cluster update failures requiring manual intervention. Only use custom AMIs if you have specific compatibility requirements and thoroughly understand EKS node bootstrapping processes. We recommend testing in a non-production environment before implementation.

**Accepted `capacity_type` values**: build groups run through the same validation as [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config), so as of 3.25.3 the CLI rejects the Azure spellings `Regular` and `Spot` on an AWS Rack before the update is submitted. Those values never worked on AWS, because EKS accepts only `ON_DEMAND` and `SPOT`. The check applies to the configuration supplied in the current command and does not re-validate what the Rack already has stored.

## Setting Parameters
To set the `additional_build_groups_config` parameter, there are several methods:

### Using a JSON File (Recommended)
```bash
$ convox rack params set additional_build_groups_config=/path/to/build-config.json -r rackName
Updating parameters... OK
```

The JSON file should be structured as follows:
```json
[
  {
    "type": "c5.2xlarge",
    "disk": 100,
    "capacity_type": "SPOT",
    "min_size": 0,
    "max_size": 5,
    "label": "app-build"
  }
]
```

> **Important Note on AWS Rate Limits**: When adding or removing multiple node groups, it's recommended to modify no more than three node groups at a time to avoid hitting AWS API rate limits. If you receive a rate limit error during an update run the parameter set command again. The operation will resume from where it left off, creating the remaining node groups without duplicating the ones that were already successfully created.

### Using a Raw JSON String
```bash
$ convox rack params set 'additional_build_groups_config=[{"type":"c5.2xlarge","disk":100,"capacity_type":"SPOT","min_size":0,"max_size":5,"label":"app-build"}]' -r rackName
Updating parameters... OK
```

## Changing an Existing Build Group

Editing an entry in `additional_build_groups_config` either updates the build node group in place or replaces it, under the same rules as [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config).

| Field changed | Result |
|---------------|--------|
| `tags` | Updated in place, no replacement. Build groups accept the same comma-separated `tags` value as `additional_node_groups_config` |
| `type`, `disk`, `capacity_type`, `ami_id` | The build group is replaced. The replacement build group is created and becomes ready before the old one is removed |
| `min_size`, `max_size`, `label` | Updated in place, no replacement |

Both of those behaviors are new in 3.25.3. Build groups received the same freeze on tag-driven replacement and the same create-first replacement ordering as workload node groups. Before 3.25.3, editing `tags` replaced the build group, and every replacement removed the old build group before creating its replacement.

Changing the build node IAM role replaces every build group at once. Build groups use a dedicated build node role when [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled) and [build_node_minimal_role_enabled](/configuration/rack-parameters/aws/build_node_minimal_role_enabled) are both `true`, and the standard node role otherwise, so toggling either parameter while the other is `true` swaps the role and replaces the build groups.

The pre-apply scale-up that lets you raise `min_size` above a running node group's current node count applies to `additional_node_groups_config` and not to build groups. Build groups do not need it: Terraform sends a new desired size along with the new minimum for a build group, so EKS accepts the change.

## Directing Build Pods to Specific Node Groups
To direct build pods to specific node groups, use the `BuildLabels` app parameter:

```bash
$ convox apps params set BuildLabels=convox.io/label=app-build -a <app>
```

This ensures that build processes for the specified app will run on nodes with the `convox.io/label: app-build` label.

## Customizing Build Pod Resources
You can also specify resource requirements for build pods:

```bash
$ convox apps params set BuildCpu=250 BuildMem=1024 -a <app>
```

This sets the CPU request to 250 millicores (0.25 vCPU) and memory request to 1024MB (1GB) for build pods.

## Architecture Compatibility

Build groups may use a different CPU architecture from the Rack's [node_type](/configuration/rack-parameters/aws/node_type). Convox selects an arm64 or x86 EKS AMI for each build group from that build group's instance type, so an x86 Rack can carry ARM build nodes and the reverse.

What matters is where each App's Builds land. A Build that runs on an ARM build node produces an ARM image, which will not run on x86 nodes, and the reverse. On a mixed-architecture Rack, use [BuildArch](/configuration/app-parameters/aws/BuildArch) to pin each App's image to the right architecture, and `nodeSelectorLabels` to pin its Services to matching nodes. The Build pod is only sent to a build node of that architecture when [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) is `true`. With the default `false`, the pod schedules on any available node and the Build is emulated. See [Workload Placement](/configuration/scaling/workload-placement) for placement strategies.

## Additional Information
Combining the `additional_build_groups_config` parameter with app-specific `BuildLabels` configuration provides:

1. **Isolation**: Build processes won't interfere with production workloads.
2. **Cost Efficiency**: You can use spot instances for build processes, which are typically tolerant of interruptions.
3. **Resource Optimization**: Custom instance types can be selected based on build requirements.
4. **Scaling Flexibility**: Build node groups can scale based on demand, potentially scaling to zero when no builds are running.

Build nodes configured with larger instance types, more memory, or faster disk I/O can significantly improve build performance for large applications, potentially reducing build times and improving developer productivity.
