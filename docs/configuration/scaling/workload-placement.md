---
title: "Workload Placement"
slug: workload-placement
url: /configuration/scaling/workload-placement
---

# Workload Placement

Convox provides powerful tools to control where your applications and build processes run within your Kubernetes cluster. By leveraging node group configurations and service placement rules, you can optimize resource usage, improve cost efficiency, and ensure the right workloads run on the right infrastructure.

> Workload Placement is available on AWS and Azure racks.

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

These parameters allow you to specify:
- Instance types (EC2 instance types on AWS, VM sizes on Azure)
- Disk sizes
- Capacity types (on-demand vs. spot)
- Scaling parameters
- Custom labels for workload targeting
- Unique IDs for node group preservation across updates
- Provider tags for cost allocation and resource organization

> These configurations are independent of each other. You can use either one or both depending on your needs. If you only configure additional node groups, builds will continue using the rack's primary build node (if [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) is set on AWS) or the primary rack nodes. If you only configure build node groups, your services will continue running on the standard rack nodes while builds will be isolated according to your build configuration.

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

### Node Group Configuration Options

Each node group configuration supports the following fields:

| Field | Required | Description | Default |
|-------|----------|-------------|---------|
| `id` | No | Unique integer identifier for the node group | Auto-generated |
| `type` | Yes | The instance type to use (AWS EC2 type or Azure VM size) | |
| `disk` | No | The disk size in GB for the nodes | Same as main node disk |
| `capacity_type` | No | Whether to use on-demand or spot instances | `ON_DEMAND` |
| `min_size` | No | Minimum number of nodes | 1 |
| `max_size` | No | Maximum number of nodes | 100 |
| `label` | No | Custom label value for the node group. Applied as `convox.io/label: <label-value>` | None |
| `tags` | No | Custom provider tags as comma-separated key-value pairs | None |
| `dedicated` | No | When `true`, only services with matching node group labels will be scheduled on these nodes | `false` |
| `ami_id` | No | Custom AMI ID to use (AWS only) | EKS-optimized AMI |
| `zones` | No | Comma-separated list of availability zones (Azure only) | None |

#### About the `id` field

The `id` field provides important benefits:
- Preserves node group identity during configuration updates
- Prevents unnecessary recreation of node groups
- Allows for stable references when targeting specific node groups
- Reduces downtime during configuration changes

Without the `id` field, Convox generates a random identifier that changes when the configuration is updated, potentially causing unnecessary node group recreation.

### App-level Configuration

At the application level, you can control where specific workloads run:

- `BuildArch`: Directs build pods to build nodes matching a specific CPU architecture (`amd64` or `arm64`)
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

### Optimizing for Cost and Performance

This example creates a cost-optimized infrastructure with dedicated node pools for different workload types.

1. **Rack Configuration**:
   ```bash
   $ convox rack params set additional_node_groups_config=/path/to/node-groups.json -r production
   $ convox rack params set additional_build_groups_config=/path/to/build-groups.json -r production
   ```

   Node groups config (AWS):
   ```json
   [
     {
       "id": 101,
       "type": "t3.medium",
       "capacity_type": "ON_DEMAND",
       "min_size": 1,
       "max_size": 5,
       "label": "critical-services",
       "tags": "department=frontend,environment=production,cost-center=web"
     },
     {
       "id": 102,
       "type": "c5.large",
       "capacity_type": "SPOT",
       "min_size": 0,
       "max_size": 10,
       "label": "batch-workers",
       "tags": "department=data,environment=production,cost-center=analytics"
     }
   ]
   ```

   Node groups config (Azure):
   ```json
   [
     {
       "id": 101,
       "type": "Standard_D4s_v3",
       "capacity_type": "ON_DEMAND",
       "min_size": 1,
       "max_size": 5,
       "label": "critical-services",
       "tags": "department=frontend,environment=production,cost-center=web"
     },
     {
       "id": 102,
       "type": "Standard_E4s_v3",
       "capacity_type": "SPOT",
       "min_size": 0,
       "max_size": 10,
       "label": "batch-workers",
       "tags": "department=data,environment=production,cost-center=analytics"
     }
   ]
   ```

   Build groups config (AWS):
   ```json
   [
     {
       "type": "c5.xlarge",
       "capacity_type": "SPOT",
       "min_size": 0,
       "max_size": 5,
       "label": "app-build"
     }
   ]
   ```

   Build groups config (Azure):
   ```json
   [
     {
       "type": "Standard_D8s_v3",
       "capacity_type": "SPOT",
       "min_size": 0,
       "max_size": 5,
       "label": "app-build"
     }
   ]
   ```

2. **Application Configuration**:
   ```bash
   $ convox apps params set BuildLabels=convox.io/label=app-build -a myapp
   $ convox apps params set BuildCpu=1000 BuildMem=4096 -a myapp
   ```

3. **Service Configuration** (in `convox.yml`):
   ```yaml
   services:
     web:
       build: .
       port: 3000
       nodeSelectorLabels:
         convox.io/label: critical-services
     worker:
       build: ./worker
       nodeSelectorLabels:
         convox.io/label: batch-workers
   ```

This configuration creates:
- On-demand nodes for critical services like web frontends
- Spot instance nodes for batch processing workloads
- Separate spot instance nodes optimized for build processes
- Provider tags for accurate cost attribution across teams and environments

### Isolating High-Priority Workloads

To create dedicated node groups that exclusively run specific services:

1. **Create a JSON file for your node group configuration**:

   AWS:
   ```json
   [
     {
       "id": 103,
       "type": "m5.large",
       "capacity_type": "ON_DEMAND",
       "min_size": 2,
       "max_size": 5,
       "label": "database-workers",
       "dedicated": true,
       "tags": "environment=production,team=datastore,criticality=high"
     }
   ]
   ```

   Azure:
   ```json
   [
     {
       "id": 103,
       "type": "Standard_E4s_v3",
       "capacity_type": "ON_DEMAND",
       "min_size": 2,
       "max_size": 5,
       "label": "database-workers",
       "dedicated": true,
       "tags": "environment=production,team=datastore,criticality=high"
     }
   ]
   ```

2. **Apply the configuration**:
   ```bash
   $ convox rack params set additional_node_groups_config=/path/to/dedicated-nodes.json -r production
   ```

3. **Service Configuration** (in `convox.yml`):
   ```yaml
   services:
     db-processor:
       build: ./processor
       nodeSelectorLabels:
         convox.io/label: database-workers
   ```

With `dedicated:true`, only services that explicitly select the node group will run on it, ensuring isolation for sensitive workloads.

### Mixed ARM/x86 Architecture

This example sets up an x86 primary rack with ARM worker nodes and ARM build nodes, allowing cost-optimized Graviton instances for selected apps while keeping x86 available for compatibility-sensitive workloads.

> Mixed architecture support is available on AWS racks running version 3.24.1 or later.

1. **Add ARM worker nodes** (dedicated so only targeted apps land on them):

   ```json
   [
     {
       "id": 104,
       "type": "t4g.medium",
       "capacity_type": "ON_DEMAND",
       "min_size": 1,
       "max_size": 5,
       "label": "arm-workers",
       "dedicated": true,
       "tags": "architecture=arm64,environment=production"
     }
   ]
   ```

   ```bash
   $ convox rack params set additional_node_groups_config=/path/to/arm-nodes.json -r production
   ```

2. **Add ARM build nodes** so ARM apps build natively without emulation:

   ```json
   [
     {
       "id": 201,
       "type": "t4g.medium",
       "capacity_type": "ON_DEMAND",
       "min_size": 1,
       "max_size": 2
     }
   ]
   ```

   ```bash
   $ convox rack params set additional_build_groups_config=/path/to/arm-build-nodes.json -r production
   ```

3. **Configure each ARM app** to build on the correct architecture:

   ```bash
   $ convox apps params set BuildArch=arm64 -a myapp
   ```

4. **Target the app to ARM workers** in `convox.yml`:

   ```yaml
   services:
     web:
       build: .
       port: 3000
       nodeSelectorLabels:
         convox.io/label: arm-workers
   ```

5. **Deploy**:

   ```bash
   $ convox deploy -a myapp
   ```

The build runs on an ARM build node (via `BuildArch=arm64`), producing a native ARM binary. The resulting pods run on the dedicated ARM worker nodes (via `nodeSelectorLabels`). Apps without `BuildArch` continue building and running on x86 nodes as before.

**Reverse direction** works the same way: if your primary rack uses ARM (e.g., `node_type=t4g.medium`), add x86 additional groups and set `BuildArch=amd64` on apps that need Intel/AMD compatibility.

**Key considerations:**
- `BuildArch` is per-app, not per-service. Apps with services targeting different architectures should be split into separate apps.
- Convox system images (including Fluentd) are multi-arch manifests and run natively on both architectures with no configuration.
- The `kubernetes.io/arch` label is set automatically by the kubelet, so this works on AWS, Azure, and GCP without provider-specific setup.

For full `BuildArch` parameter details, see [BuildArch](/configuration/app-parameters/aws/BuildArch).

### Flexible Configuration Options

Convox allows you to implement different levels of customization based on your needs:

1. **Build Isolation Only**: Configure only `additional_build_groups_config` to isolate build processes while keeping services on standard nodes:
   ```bash
   $ convox rack params set additional_build_groups_config=/path/to/build-groups.json -r production
   $ convox apps params set BuildLabels=convox.io/label=app-build -a myapp
   ```

2. **Service Placement Only**: Configure only `additional_node_groups_config` to customize service placement while letting builds run on standard nodes:
   ```bash
   $ convox rack params set additional_node_groups_config=/path/to/node-groups.json -r production
   ```
   In your `convox.yml`:
   ```yaml
   services:
     web:
       nodeSelectorLabels:
         convox.io/label: critical-services
   ```

3. **Complete Workload Management**: Implement both configurations for full control over placement of both services and build processes.

## Best Practices

1. **CPU Architecture — Single or Mixed**:
   - A rack's primary nodes define the default architecture. Additional node groups can use a different architecture to create a mixed ARM/x86 rack.
   - On AWS, Graviton instances (e.g. `t4g`, `c7g`, `m7g`) are ARM. Standard instances (e.g. `t3`, `c5`, `m5`) are x86. You can mix architectures by adding additional node groups and build groups with different instance families. See [node_type](/configuration/rack-parameters/aws/node_type#cpu-architecture-x86-vs-arm) for the full list of supported instance families.
   - On Azure, only x86-based VM SKUs are currently supported. ARM-based VM SKUs are not available. See [node_type](/configuration/rack-parameters/azure/node_type) for details.
   - **Mixed architecture requires `BuildArch`**: When running mixed-architecture node groups, use the [`BuildArch`](/configuration/app-parameters/aws/BuildArch) app parameter to direct each app's builds to build nodes matching its target architecture. Without `BuildArch`, builds run on any available build node and may produce binaries for the wrong architecture.
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

### Node Group Scaling
If nodes aren't scaling as expected:
- Verify min/max settings are appropriate
- Check that instance types or VM sizes are available in your region
- Monitor for cloud provider service quotas that might limit scaling

### Node Group Preservation Issues
If node groups are being recreated unexpectedly:
- Ensure each node group has a unique `id` field
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
