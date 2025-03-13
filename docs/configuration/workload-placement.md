---
title: "Workload Placement"
draft: false
slug: Workload Placement
url: /configuration/workload-placement
---

# Workload Placement

Convox provides powerful tools to control where your applications and build processes run within your Kubernetes cluster. By leveraging node group configurations and service placement rules, you can optimize resource usage, improve cost efficiency, and ensure the right workloads run on the right infrastructure.

## Overview

Workload placement in Convox is achieved through these key features:

1. **Custom Node Groups**: Define specialized node pools with specific instance types, scaling parameters, and labels.
2. **Node Selectors**: Direct specific services or build processes to appropriate node groups.
3. **Dedicated Node Pools**: Isolate workloads by creating exclusive node groups for particular services.

These capabilities allow for sophisticated infrastructure optimization strategies, such as:

- Separating production services from build processes
- Using cost-effective spot instances for non-critical workloads
- Optimizing instance types for specific workload profiles
- Creating high-performance node groups for specialized services

## Configuration Components

### Rack-level Configuration

At the rack level, you can define custom node groups:

- [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config): Creates general-purpose node groups
- [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config): Creates node groups specifically for build processes

These parameters allow you to specify instance types, disk sizes, capacity types (on-demand vs. spot), scaling parameters, and custom labels for your node groups.

> **Note**: These configurations are independent of each other. You can use either one or both depending on your needs. If you only configure additional node groups, builds will continue using the rack's primary build node (if [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) is set) or the primary rack nodes. If you only configure build node groups, your services will continue running on the standard rack nodes while builds will be isolated according to your build configuration.

### Setting Rack Parameters with JSON Files

While you can set configuration directly using a JSON string, most users find it more manageable to use a JSON file, especially for complex configurations.

#### Using a JSON File for Node Groups

Create a JSON file (e.g., `node-groups.json`) with your configuration:

```json
[
  {
    "type": "t3.medium",
    "capacity_type": "ON_DEMAND",
    "min_size": 1,
    "desired_size": 2,
    "max_size": 5,
    "label": "critical-services"
  },
  {
    "type": "c5.large",
    "capacity_type": "SPOT",
    "min_size": 0,
    "desired_size": 1,
    "max_size": 10,
    "label": "batch-workers",
    "disk": 100
  }
]
```

Then apply the configuration using:

```bash
$ convox rack params set additional_node_groups_config=/path/to/node-groups.json -r rackName
```

> **Important Note on AWS Rate Limits**: When adding or removing multiple node groups, it's recommended to modify no more than three node groups at a time to avoid hitting AWS API rate limits. If you receive a rate limit error during an update simply run the parameter set command again. The operation will resume from where it left off, creating the remaining node groups without duplicating the ones that were already successfully created.

#### Using a JSON File for Build Node Groups

Similarly, create a JSON file (e.g., `build-groups.json`) for build node configuration:

```json
[
  {
    "type": "c5.xlarge",
    "capacity_type": "SPOT",
    "min_size": 0,
    "desired_size": 0,
    "max_size": 3,
    "label": "app-build",
    "disk": 100
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
$ convox rack params set 'additional_node_groups_config=[{"type":"t3.medium","capacity_type":"ON_DEMAND","min_size":1,"desired_size":2,"max_size":5,"label":"critical-services"}]' -r rackName
```

```bash
$ convox rack params set 'additional_build_groups_config=[{"type":"c5.xlarge","capacity_type":"SPOT","min_size":0,"desired_size":0,"max_size":3,"label":"app-build","disk":100}]' -r rackName
```

This approach is useful for automation scripts or when making quick changes, though it becomes unwieldy for more complex configurations.

### App-level Configuration

At the application level, you can control where specific workloads run:

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

## Implementation Examples

### Optimizing for Cost and Performance

This example creates a cost-optimized infrastructure with dedicated node pools for different workload types:

1. **Rack Configuration**:
   ```html
   $ convox rack params set additional_node_groups_config=/path/to/node-groups.json -r production
   $ convox rack params set additional_build_groups_config=/path/to/build-groups.json -r production
   ```

2. **Application Configuration**:
   ```html
   $ convox apps params set BuildLabels=convox.io/label=app-build -a myapp
   $ convox apps params set BuildCpu=1024 BuildMem=4096 -a myapp
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

### Isolating High-Priority Workloads

To create dedicated node groups that exclusively run specific services:

1. **Create a JSON file for your node group configuration**:
   ```json
   [
     {
       "type": "m5.large",
       "capacity_type": "ON_DEMAND",
       "min_size": 2,
       "desired_size": 3,
       "max_size": 5,
       "label": "database-workers",
       "dedicated": true
     }
   ]
   ```

2. **Apply the configuration**:
   ```html
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

1. **Match Node Resources to Workload Requirements**:
   - Use compute-optimized instances (c-type) for CPU-intensive workloads
   - Use memory-optimized instances (r-type) for memory-intensive workloads
   - Use general-purpose instances (m-type or t-type) for balanced workloads

2. **Cost Optimization**:
   - Use spot instances for interruptible workloads like batch processing
   - Use on-demand instances for critical production services
   - Set appropriate min/max scaling parameters to avoid over-provisioning

3. **Build Process Optimization**:
   - Configure build nodes with higher CPU and memory for faster builds
   - Use spot instances for builds to reduce costs
   - Set `min_size: 0` to allow build nodes to scale down when not in use

4. **Service Isolation**:
   - Use the `dedicated` flag for node groups that need strict isolation
   - Separate services with conflicting resource profiles into different node groups

5. **Label Consistency**:
   - Maintain a consistent labeling strategy across your infrastructure
   - Document the purpose and characteristics of each node group

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
- Verify min/max/desired settings are appropriate
- Check that instance types are available in your region
- Monitor for AWS service quotas that might limit scaling

## Conclusion

Effective workload placement is a powerful tool for optimizing your Convox infrastructure. By leveraging custom node groups and service placement rules, you can create an infrastructure that balances performance, cost, and isolation requirements for your specific application needs.

For more detailed information, refer to:
- [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config)
- [additional_build_groups_config](/configuration/rack-parameters/aws/additional_build_groups_config)
- [BuildLabels](/configuration/app-parameters/aws/BuildLabels)
