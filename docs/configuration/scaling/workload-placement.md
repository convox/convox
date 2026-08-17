---
title: "Workload Placement"
description: "Control where Convox apps and build processes run with custom node groups, node selectors, and dedicated node pools on AWS, Azure, and GCP racks."
slug: workload-placement
url: /configuration/scaling/workload-placement
---

# Workload Placement

Convox provides powerful tools to control where your applications and build processes run within your Kubernetes cluster. By leveraging node group configurations and service placement rules, you can optimize resource usage, improve cost efficiency, and ensure the right workloads run on the right infrastructure.

> Workload Placement is available on AWS, Azure, and GCP racks. GCP supports additional node groups only, not build node groups.

Reach for workload placement when the default single node group is no longer the right fit: when you want to run builds on separate hardware from production services, use cheaper spot instances for non-critical work, match instance types to specific workload profiles, isolate sensitive services on dedicated nodes, or mix CPU architectures within one rack. If your apps run fine on the rack's standard nodes, you do not need any of this.

This page is organized so you can decide first, configure second. The strategies and configuration components below explain the moving parts and the field options. The JSON examples and implementation walkthroughs that follow give you copy-ready configurations once you know what you want.

## Workload Placement Strategies

Workload placement in Convox is achieved through these key features:

1. **Custom Node Groups**: Define specialized node pools with specific instance types, scaling parameters, labels, and tags.
2. **Node Selectors**: Direct specific services or build processes to appropriate node groups.
3. **Dedicated Node Pools**: Isolate workloads by creating exclusive node groups for particular services.

These capabilities allow for sophisticated infrastructure optimization strategies, such as:

- Separating production services from build processes
- Using cost-effective spot instances for non-critical workloads
- Optimizing instance types for specific workload profiles
- Creating high-performance node groups for specialized services
- Tracking resource usage and costs with provider tags

## Configuration Components

### Rack-level Configuration

At the rack level, you can define custom node groups using provider-specific rack parameters:

**AWS:**
- [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config): Creates general-purpose node groups
- [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config): Creates node groups specifically for build processes

**Azure:**
- [`additional_node_groups_config`](/configuration/rack-parameters/azure/additional_node_groups_config): Creates general-purpose node pools
- [`additional_build_groups_config`](/configuration/rack-parameters/azure/additional_build_groups_config): Creates node pools specifically for build processes

**GCP:**
- [`additional_node_groups_config`](/configuration/rack-parameters/gcp/additional_node_groups_config): Creates general-purpose node pools, GPU node pools (via the `gpu_type` and `gpu_count` fields), and single-host Cloud TPU node pools (via a TPU machine type and the `tpu_topology` field)

These parameters allow you to specify:
- Instance types (EC2 instance types on AWS, VM sizes on Azure)
- Disk sizes
- Capacity types (on-demand vs. spot)
- Scaling parameters
- Custom labels for workload targeting
- Unique IDs for node group preservation across updates
- Provider tags for cost allocation and resource organization

> These configurations are independent of each other. You can use either one or both depending on your needs. If you only configure additional node groups, builds will continue using the rack's primary build node (if [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) is set on AWS) or the primary rack nodes. If you only configure build node groups, your services will continue running on the standard rack nodes while builds will be isolated according to your build configuration.

### Node Group Configuration Options

Each node group configuration supports the following fields:

| Field | Required | Description | Default |
|-------|----------|-------------|---------|
| `id` | No | Unique integer identifier for the node group | The entry's position in the array |
| `type` | Yes | The instance type to use (AWS EC2 type, Azure VM size, or GCP machine type) | |
| `disk` | No | The disk size in GB for the nodes | Same as main node disk |
| `capacity_type` | No | Whether to use on-demand or spot instances. AWS accepts `ON_DEMAND` or `SPOT`; Azure and GCP additionally accept `Regular` and `Spot`. Values are matched exactly. | `ON_DEMAND` |
| `min_size` | No | Minimum number of nodes | 1 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value for the node group. Applied as `convox.io/label: <label-value>` | On AWS, `custom` for node groups and `custom-build` for build groups. On Azure, `custom-build` for build groups; an Azure node group with no `label` gets no `convox.io/label`. On GCP, no `convox.io/label` unless `dedicated` is `true`, which applies `custom` |
| `tags` | No | Custom provider tags as comma-separated key-value pairs | None |
| `dedicated` | No | When `true`, only services with matching node group labels will be scheduled on these nodes | `false` |
| `ami_id` | No | Custom AMI ID to use (AWS only) | EKS-optimized AMI |
| `zones` | No | Comma-separated list of availability zones (Azure and GCP). Requires a convox CLI at 3.25.3 or newer; earlier CLIs drop the field before it reaches Terraform. | None |

GCP node groups support additional fields (`gpu_type`, `gpu_count`, `tpu_topology`, `disk_type`), and `min_size`/`max_size` apply per zone (GCP racks use regional clusters) rather than as totals. See [Rack Parameters: additional_node_groups_config (GCP)](/configuration/rack-parameters/gcp/additional_node_groups_config).

#### About the `id` field

The `id` field provides important benefits:
- Fixes each node group's identity across configuration edits, so adding or reordering entries does not renumber the groups you already have
- Allows for stable references when targeting specific node groups

Without the `id` field, each node group is keyed by its position in the array, so reordering or removing an entry shifts the keys and recreates groups you did not intend to change.

The `id` field does not prevent a node group from being replaced when you change the group's own shape, such as its instance type, disk size, or capacity type. What happens during that replacement depends on the provider: on AWS, as of 3.25.3, the replacement node group is created and reaches ready before the old one is removed; on Azure the pool rotates through a temporary pool; on GCP the pool is destroyed before it is recreated. See the provider's `additional_node_groups_config` page for the per-field detail: [AWS](/configuration/rack-parameters/aws/additional_node_groups_config), [Azure](/configuration/rack-parameters/azure/additional_node_groups_config), [GCP](/configuration/rack-parameters/gcp/additional_node_groups_config).

### Setting Rack Parameters with JSON Files

While you can set configuration directly using a JSON string, most users find it more manageable to use a JSON file, especially for complex configurations.

#### Using a JSON File for Node Groups

Create a JSON file (e.g., `node-groups.json`) with your configuration. The `type` field uses EC2 instance types on AWS and VM sizes on Azure.

**AWS example:**
```json
[
  {
    "id": 101,
    "type": "t3.medium",
    "capacity_type": "ON_DEMAND",
    "min_size": 1,
    "max_size": 5,
    "label": "critical-services",
    "tags": "environment=production,team=frontend"
  },
  {
    "id": 102,
    "type": "c5.large",
    "capacity_type": "SPOT",
    "min_size": 0,
    "max_size": 10,
    "label": "batch-workers",
    "disk": 100,
    "tags": "environment=production,team=data,workload=batch"
  }
]
```

**Azure example:**
```json
[
  {
    "id": 101,
    "type": "Standard_D4s_v3",
    "capacity_type": "ON_DEMAND",
    "min_size": 1,
    "max_size": 5,
    "label": "critical-services",
    "tags": "environment=production,team=frontend"
  },
  {
    "id": 102,
    "type": "Standard_E4s_v3",
    "capacity_type": "SPOT",
    "min_size": 0,
    "max_size": 10,
    "label": "batch-workers",
    "disk": 100,
    "tags": "environment=production,team=data,workload=batch"
  }
]
```

Note the use of:
- The `id` field to uniquely identify each node group
- The `tags` field to apply provider resource tags for organization and cost tracking

Then apply the configuration using:

```bash
$ convox rack params set additional_node_groups_config=/path/to/node-groups.json -r rackName
```

> **Important Note on AWS Rate Limits**: On AWS, when adding or removing multiple node groups, modify no more than three node groups at a time to avoid hitting AWS API rate limits. If you receive a rate limit error during an update, run the parameter set command again. The operation will resume from where it left off, creating the remaining node groups without duplicating the ones that were already successfully created.

#### Using a JSON File for Build Node Groups

Similarly, create a JSON file (e.g., `build-groups.json`) for build node configuration:

**AWS example:**
```json
[
  {
    "id": 201,
    "type": "c5.xlarge",
    "capacity_type": "SPOT",
    "min_size": 0,
    "max_size": 3,
    "label": "app-build",
    "disk": 100,
    "tags": "environment=build,team=devops"
  }
]
```

**Azure example:**
```json
[
  {
    "id": 201,
    "type": "Standard_D8s_v3",
    "capacity_type": "SPOT",
    "min_size": 0,
    "max_size": 3,
    "label": "app-build",
    "disk": 100,
    "tags": "environment=build,team=devops"
  }
]
```

Apply it with:

```bash
$ convox rack params set additional_build_groups_config=/path/to/build-groups.json -r rackName
```

#### Using a Single JSON String (Alternative Approach)

If you prefer to set configuration directly in the command line without creating a file, you can use a JSON string:

```bash
$ convox rack params set 'additional_node_groups_config=[{"id":101,"type":"t3.medium","capacity_type":"ON_DEMAND","min_size":1,"max_size":5,"label":"critical-services","tags":"environment=production,team=frontend"}]' -r rackName
```

```bash
$ convox rack params set 'additional_build_groups_config=[{"id":201,"type":"c5.xlarge","capacity_type":"SPOT","min_size":0,"max_size":3,"label":"app-build","disk":100,"tags":"environment=build,team=devops"}]' -r rackName
```

This approach is useful for automation scripts or when making quick changes, though it becomes unwieldy for more complex configurations.

### App-level Configuration

At the application level, you can control where specific workloads run:

- `BuildArch`: Pins the App's built image to a CPU architecture (`amd64` or `arm64`), and directs the build Pod to a matching build node when [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled). Takes effect on Racks running 3.25.3 or later.
- `BuildLabels`: Directs build pods to specific node groups
- `BuildCpu` and `BuildMem`: Sets resource requests for build pods
- `nodeSelectorLabels` in `convox.yml`: Directs service pods to specific node groups

### Service-level Configuration

In your `convox.yml` file, you can specify node selectors for each service:

```yaml
services:
  web:
    nodeSelectorLabels:
      convox.io/label: app-workers
  worker:
    nodeSelectorLabels:
      convox.io/label: batch-workers
```

You can also specify nodeAffinityLabels with weights to specify preferences of where to place services. The `node.kubernetes.io/instance-type` label uses EC2 instance types on AWS or Azure VM sizes on Azure:

AWS example:
```yaml
services:
  web:
    nodeAffinityLabels:
      - weight: 1
        label: node.kubernetes.io/instance-type
        value: t3a.medium
      - weight: 10
        label: node.kubernetes.io/instance-type
        value: t3a.large
```

Azure example:
```yaml
services:
  web:
    nodeAffinityLabels:
      - weight: 1
        label: node.kubernetes.io/instance-type
        value: Standard_D2s_v3
      - weight: 10
        label: node.kubernetes.io/instance-type
        value: Standard_D4s_v3
```

Weights will be summed for all matching labels and the node with the highest weight will have the service scheduled on it.

You can combine the two options as well:

```yaml
services:
  web:
    nodeSelectorLabels:
      convox.io/label: app-workers
    nodeAffinityLabels:
      - weight: 1
        label: node.kubernetes.io/instance-type
        value: t3a.medium
      - weight: 10
        label: node.kubernetes.io/instance-type
        value: t3a.large
```

In this case, the service will definitely be scheduled on the `app-workers` group, preferably on a `t3a.large` instance, or if not on a `t3a.medium` instance, or if not, then any other instance in the group. Use the equivalent Azure VM sizes when running on Azure racks.

## Implementation Examples

For copy-ready, end-to-end walkthroughs, see [Workload Placement Examples](/configuration/scaling/workload-placement-examples). It covers optimizing for cost and performance, isolating high-priority workloads, mixed ARM/x86 architecture, and flexible configuration options.

## Best Practices

1. **CPU Architecture (Single or Mixed)**:
   - A rack's primary nodes define the default architecture. Additional node groups can use a different architecture to create a mixed ARM/x86 rack.
   - On AWS, Graviton instances (e.g. `t4g`, `c7g`, `m7g`) are ARM. Standard instances (e.g. `t3`, `c5`, `m5`) are x86. You can mix architectures by adding additional node groups and build groups with different instance families. See [node_type](/configuration/rack-parameters/aws/node_type#cpu-architecture-x86-vs-arm) for the full list of supported instance families.
   - On Azure, only x86-based VM SKUs are currently supported. ARM-based VM SKUs are not available. See [node_type](/configuration/rack-parameters/azure/node_type) for details.
   - **Mixed architecture with managed node groups requires `BuildArch`**: When running mixed-architecture managed node groups, use the [`BuildArch`](/configuration/app-parameters/aws/BuildArch) app parameter (Racks running 3.25.3 or later) to pin each App's image to its target architecture. Without it, builds may produce images for the wrong architecture. On a Karpenter Rack with `karpenter_arch=amd64,arm64` this is not needed: builds produce a multi-architecture image index. See [Architecture Selection and Mixed-Architecture Racks](/configuration/scaling/karpenter#architecture-selection-and-mixed-architecture-racks).
   - **Convox system images are multi-arch**: System components (including Fluentd) are published as multi-arch Docker manifests and run natively on both x86 and ARM nodes with no configuration.

2. **Match Node Resources to Workload Requirements**:
   - On AWS: use compute-optimized instances (`c5`, `c6i`) for CPU-intensive workloads, memory-optimized (`r5`, `r6i`) for memory-intensive workloads, and general-purpose (`m5`, `t3`) for balanced workloads
   - On Azure: use compute-optimized VMs (`Standard_F` series) for CPU-intensive workloads, memory-optimized (`Standard_E` series) for memory-intensive workloads, and general-purpose (`Standard_D` series) for balanced workloads

3. **Cost Optimization**:
   - Use spot instances for interruptible workloads like batch processing
   - Use on-demand instances for critical production services
   - Set appropriate min/max scaling parameters to avoid over-provisioning
   - Apply tags to track costs by team, environment, or application

4. **Build Process Optimization**:
   - Configure build nodes with higher CPU and memory for faster builds
   - Use spot instances for builds to reduce costs
   - Set `min_size: 0` to allow build nodes to scale down when not in use

5. **Service Isolation**:
   - Use the `dedicated` flag for node groups that need strict isolation
   - Separate services with conflicting resource profiles into different node groups

6. **Node Group Identity Management**:
   - Always assign a unique `id` to each node group
   - Use consistent, meaningful IDs (e.g., 100-199 for production, 200-299 for builds)
   - Document your ID allocation to avoid conflicts

7. **Tagging Strategy**:
   - Develop a consistent tagging convention for all node groups
   - Include tags for environment, team, cost center, and workload type
   - Align tags with your organization's cloud provider tagging policy

## Troubleshooting

### Build Failures Due to Node Selection
If builds fail with scheduling errors, verify:
- The build node group exists and has the correct labels
- The `BuildLabels` parameter matches the node group's labels
- There are nodes available that match the label criteria

### Service Deployment Issues
If services won't deploy, check:
- Node selector labels in `convox.yml` match existing node groups
- The referenced node groups have available capacity
- Resource requests in the service definition can be satisfied by the node group

### Deploy Rejected for a Missing Node Pool
On a Rack running Karpenter at `3.25.4` or later, a deploy fails immediately when a Service's `nodeSelectorLabels` names a `convox.io/nodepool` value the Rack does not have:

```
convox.yml: service "web" targets Karpenter node pool "gpu-lg", which does not exist on this rack.
  Existing pools: build, workload
```

Compare the name in `convox.yml` against the `Existing pools` list in the message. If you just added the pool, wait for the Rack update to finish and retry. Every Service in the manifest is checked regardless of replica count, so a stale pin on a Service scaled to zero blocks the App's healthy Services too. The same check applies to `convox run --node-labels` and to the `BuildLabels` App parameter. See [Node Pool Validation](/configuration/scaling/karpenter#node-pool-validation).

### Node Group Scaling
If nodes aren't scaling as expected:
- Verify min/max settings are appropriate
- Check that instance types or VM sizes are available in your region
- Monitor for cloud provider service quotas that might limit scaling

### Node Group Preservation Issues
If node groups are being recreated unexpectedly:
- Ensure each node group has a unique `id` field. An `id` stops existing groups from being renumbered when you reorder or remove entries, but it does not stop a group from being replaced when you change that group's instance type, disk size, capacity type, or AMI. The convox CLI rejects a configuration that reuses an `id`.
- Verify that you're not changing immutable fields (like capacity type)
- On AWS, check for API rate limits during updates

## Summary

Effective workload placement is a powerful tool for optimizing your Convox infrastructure. By leveraging custom node groups with preserved identities, service placement rules, and provider tagging, you can create an infrastructure that balances performance, cost, and isolation requirements for your specific application needs.

For more detailed information, refer to the provider-specific rack parameter pages:

**AWS:**
- [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config)
- [additional_build_groups_config](/configuration/rack-parameters/aws/additional_build_groups_config)

**Azure:**
- [additional_node_groups_config](/configuration/rack-parameters/azure/additional_node_groups_config)
- [additional_build_groups_config](/configuration/rack-parameters/azure/additional_build_groups_config)

**App Parameters:**
- [BuildArch](/configuration/app-parameters/aws/BuildArch)
- [BuildLabels](/configuration/app-parameters/aws/BuildLabels)

**Node Autoscaling:**
- [Karpenter](/configuration/scaling/karpenter) for pod-level node provisioning with cost optimization and scale-to-zero builds (AWS only)
