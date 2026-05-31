---
title: "Workload Placement Examples"
description: "Copy-ready Convox workload placement configurations for node groups, node selectors, and dedicated pools to optimize cost and isolate workloads."
slug: workload-placement-examples
url: /configuration/scaling/workload-placement-examples
---

# Workload Placement Examples

These worked examples give you copy-ready configurations once you know what you want. For the concepts, configuration components, and field options, see [Workload Placement](/configuration/scaling/workload-placement).

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
