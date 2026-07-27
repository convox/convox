---
title: "additional_node_groups_config"
description: "The additional_node_groups_config AWS rack parameter configures additional customized node groups for the cluster with specific instance types and labels."
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
| `type` | Yes | The EC2 instance type to use for the node group |  |
| `disk` | No | The disk size in GB for the nodes | Same as main node disk |
| `capacity_type` | No | Whether to use on-demand or spot instances. Accepts `ON_DEMAND` or `SPOT` only, matched exactly. The Azure aliases `Regular` and `Spot` are rejected | `ON_DEMAND` |
| `min_size` | No | Minimum number of nodes | 1 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value for the node group. Applied as `convox.io/label: <label-value>` | `custom` (nodes are labelled `convox.io/label: custom` when no label is set) |
| `id` | No | A unique integer identifier that fixes the node group's Terraform identity across updates | The entry's position in the array |
| `tags` | No | Custom AWS tags specified as comma-separated key-value pairs (e.g., `environment=production,team=backend`) | None |
| `ami_id`* | No | Custom AMI ID to use | EKS-optimized AMI |
| `dedicated` | No | When `true`, only services with matching node group labels will be scheduled on these nodes | `false` |

\* **Important**: Custom AMI configuration should be used with extreme caution. AMIs in EKS clusters have strict compatibility requirements, and improper configuration can lead to cluster update failures requiring manual intervention. Only use custom AMIs if you have specific compatibility requirements and thoroughly understand EKS node bootstrapping processes. We recommend testing in a non-production environment before implementation.

**Accepted `capacity_type` values**: as of 3.25.3 the CLI rejects the Azure spellings `Regular` and `Spot` on an AWS Rack before the update is submitted. Those values never worked on AWS, because EKS accepts only `ON_DEMAND` and `SPOT`, so the change moves the failure from the Terraform apply to the command itself. The check applies to the configuration supplied in the current command and does not re-validate what the Rack already has stored, so a Rack carrying a legacy value is not blocked from making unrelated parameter changes.

**Fields the AWS module ignores**: the CLI accepts `zones` and `disk_type` on every provider, but the AWS Terraform module reads neither. Setting them on an AWS Rack has no effect and produces no error.

## Setting Parameters
To set the `additional_node_groups_config` parameter, there are several methods:

If you are editing an entry that already exists rather than adding a new one, read [Changing an Existing Node Group](#changing-an-existing-node-group) below first. Changing `type`, `disk`, `capacity_type`, or `ami_id` replaces the node group, total capacity can dip during the replacement, and the group comes back at `min_size`.

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

> **Important Note on AWS Rate Limits**: When adding or removing multiple node groups, it's recommended to modify no more than three node groups at a time to avoid hitting AWS API rate limits. If you receive a rate limit error during an update run the parameter set command again. The operation will resume from where it left off, creating the remaining node groups without duplicating the ones that were already successfully created.

### Using a Raw JSON String
```bash
$ convox rack params set 'additional_node_groups_config=[{"id":101,"type":"t3.medium","disk":50,"capacity_type":"ON_DEMAND","min_size":1,"max_size":3,"label":"app-workers","tags":"environment=production,team=backend"}]' -r rackName
Updating parameters... OK
```

## Node Group Identification and Tagging

### Using the `id` Field

The `id` field is the key Terraform uses to track a node group across configuration updates. Set an explicit `id` on every entry.

- The `id` keeps a node group's identity stable, so reordering entries or removing one does not churn the node groups you did not intend to change.
- The `id` does not prevent a node group from being replaced, and never has. Which edits replace a node group is determined by the fields listed in [Changing an Existing Node Group](#changing-an-existing-node-group) below, whether or not an `id` is set.
- Without an `id`, the node group is keyed by its position in the array, and the CLI backfills ids from the stored configuration by position. Reordering or removing entries can therefore reassign identities, applying one node group's configuration to another.
- Each `id` must be unique. The CLI rejects a configuration with duplicate `id` values, and rejects one where some entries carry an `id` and others do not.

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
- Editing `tags` on an existing node group updates the node group in place and rolls its nodes. See [Changing an Existing Node Group](#changing-an-existing-node-group) below

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

## Changing an Existing Node Group

Editing an entry in `additional_node_groups_config` either updates the existing node group in place or replaces it. Which one happens depends on the field you change.

| Field changed | Result |
|---------------|--------|
| `tags` | Updated in place, no replacement. The new tags reach the nodes through a new launch template version, which EKS applies as a graceful rolling node update |
| `type`, `disk`, `capacity_type`, `ami_id` | The node group is replaced. The replacement node group is created and becomes ready before the old node group is removed |
| `min_size`, `max_size`, `label`, `dedicated` | Updated in place, no replacement |
| `id` | The node group moves to a new Terraform key, which creates a node group under the new `id` and removes the one under the old `id` |

Both the in-place `tags` update and the create-first ordering of replacements are new in 3.25.3. Before 3.25.3, editing `tags` replaced the node group, and every replacement removed the old node group before creating its replacement, taking the group's nodes down for the duration.

The pace of the rolling node update on a `tags` edit follows [node_max_unavailable_percentage](/configuration/rack-parameters/aws/node_max_unavailable_percentage). Its default is `0`, which leaves the rate to the EKS default of one node at a time.

### What Create-First Does and Does Not Guarantee

The replacement node group is created and becomes ready before the old node group is removed. That ordering is not a guarantee that total capacity holds steady throughout: in testing, the old node group's node count fell before the replacement node group appeared, so a tightly packed pool can run with fewer nodes for part of a replacing edit. Make replacing edits when the pool can tolerate reduced capacity, and leave enough headroom for pods to reschedule.

A replacement also recreates the node group at `min_size`. A node group that had autoscaled above `min_size` comes back at `min_size` and scales up again from there.

### Rack-Level Changes That Replace Every Additional Node Group

Three Rack-level inputs are part of every additional node group's identity, so changing any of them replaces all of the additional node groups at once, with the same create-first ordering:

- The [private](/configuration/rack-parameters/aws/private) parameter
- The subnet lists, [private_subnets_ids](/configuration/rack-parameters/aws/private_subnets_ids) and [public_subnets_ids](/configuration/rack-parameters/aws/public_subnets_ids)
- The node IAM role

### Raising `min_size` on a Running Node Group

Raising `min_size` above the number of nodes a group is currently running used to fail the update with an EKS validation error, because Terraform sends the new minimum without a new desired size and EKS rejects a minimum that exceeds the group's desired size. Convox now raises the group's live desired size to the new minimum first, raising the live maximum size too when the new minimum exceeds it, and waits for that scale-up to finish before applying the update, so the update succeeds. A large jump in `min_size` makes the update take longer, because the scale-up runs before the apply starts.

This handling has limits:

- It covers AWS Racks only.
- It covers `additional_node_groups_config` only. [additional_build_groups_config](/configuration/rack-parameters/aws/additional_build_groups_config) does not have the underlying problem and does not need it, and the primary node groups are outside its scope, so it does not apply to [min_on_demand_count](/configuration/rack-parameters/aws/min_on_demand_count) or [build_node_min_count](/configuration/rack-parameters/aws/build_node_min_count).
- It runs in the convox binary rather than in the Rack's Terraform modules, so it arrives with a newer convox CLI, or with a Console rebuilt against one, and not with a Rack version upgrade.

## Using Node Groups with Services
To target specific services to run on particular node groups, use the `nodeSelectorLabels` field in your `convox.yml` file:

```yaml
services:
  web:
    nodeSelectorLabels:
      convox.io/label: app-workers
```

This will ensure that the `web` service is scheduled only on nodes with the label `convox.io/label: app-workers`.

## Architecture Compatibility

Additional node groups may use a different CPU architecture from the Rack's [node_type](/configuration/rack-parameters/aws/node_type). Convox selects an arm64 or x86 EKS AMI for each node group from that node group's instance type, so x86 and ARM node groups can coexist in one Rack.

What does not follow automatically is the App image. An image built for one architecture will not run on nodes of the other. On a mixed-architecture Rack, use [BuildArch](/configuration/app-parameters/aws/BuildArch) to target each App's Builds at the right architecture, and `nodeSelectorLabels` to pin its Services to matching nodes. See [Workload Placement](/configuration/scaling/workload-placement) for placement strategies.

## Additional Information
When using dedicated node groups (with `dedicated: true`), only services with matching node selector labels will be scheduled on those nodes. This provides strong isolation for workloads with specific requirements.

For build-specific node groups, see the [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config) parameter.

Properly configured node groups can significantly improve cluster efficiency, resource utilization, and cost optimization while providing the right resource profiles for different workload types.
