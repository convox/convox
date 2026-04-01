---
title: "BuildArch"
slug: buildarch
url: /configuration/app-parameters/aws/BuildArch
---

# BuildArch

## What BuildArch Controls
The `BuildArch` app parameter directs build pods to build nodes matching a specific CPU architecture. This enables mixed-architecture racks where x86 and ARM node groups coexist, and each app builds natively on its target architecture without emulation.

When set, build pods receive a `kubernetes.io/arch` node affinity that restricts scheduling to build nodes of the specified architecture. When not set, builds run on any available build node (existing behavior).

## Default Value
By default, `BuildArch` is not set. Builds are scheduled on any available build node with no architecture preference.

## Supported Values

| Value | Architecture | Example Instance Types |
|-------|-------------|----------------------|
| `amd64` | x86/Intel/AMD | `t3.medium`, `c5.xlarge`, `m5.large` |
| `arm64` | ARM/Graviton | `t4g.medium`, `c7g.xlarge`, `m7g.large` |

## Use Cases
- **Mixed-Architecture Racks**: Run ARM worker nodes for cost-optimized workloads alongside x86 nodes for compatibility, building each app natively on its target architecture.
- **Incremental ARM Migration**: Migrate apps from x86 to ARM one at a time by adding an ARM node group, setting `BuildArch=arm64`, and targeting the app to ARM workers.
- **Avoiding Cross-Architecture Emulation**: Native builds are significantly faster and more reliable than QEMU-emulated cross-architecture builds.

## Setting the Parameter
To configure an app to build on ARM nodes:

```bash
$ convox apps params set BuildArch=arm64 -a <app>
Setting BuildArch... OK
```

To configure an app to build on x86 nodes:

```bash
$ convox apps params set BuildArch=amd64 -a <app>
Setting BuildArch... OK
```

## Viewing Current Configuration
To view the current BuildArch setting:

```bash
$ convox apps params -a <app>
NAME         VALUE
BuildArch    arm64
BuildCpu     500
BuildMem     1024
```

## Example: Mixed-Architecture Rack Setup

This walkthrough sets up a rack with x86 primary nodes and adds ARM workers and ARM build nodes.

**1. Add ARM worker nodes:**

```bash
$ convox rack params set additional_node_groups_config='[{"id":1,"type":"t4g.medium","min_size":1,"max_size":3,"label":"arm-workers","dedicated":true}]' -r rackName
```

**2. Add ARM build nodes:**

```bash
$ convox rack params set additional_build_groups_config='[{"id":1,"type":"t4g.medium","min_size":1,"max_size":2}]' -r rackName
```

**3. Configure the app to build on ARM:**

```bash
$ convox apps params set BuildArch=arm64 -a myapp
```

**4. Target the app to ARM workers in `convox.yml`:**

```yaml
services:
  web:
    build: .
    port: 3000
    nodeSelectorLabels:
      convox.io/label: arm-workers
```

**5. Deploy:**

```bash
$ convox deploy -a myapp
```

The build pod runs on an ARM build node (due to `BuildArch=arm64`), and the resulting pods run on the ARM worker nodes (due to `nodeSelectorLabels`).

## Important Considerations

- **Per-App, Not Per-Service**: `BuildArch` applies to the entire app. If an app has services targeting different architectures, split them into separate apps.
- **Requires Matching Build Nodes**: The rack must have build nodes of the specified architecture. Without matching build nodes, build pods will remain pending.
- **Cross-Provider**: Uses the `kubernetes.io/arch` Kubernetes label, which works on AWS, Azure, and GCP.
- **Fluentd**: Convox system images (including Fluentd) are multi-arch manifests and run natively on both architectures with no additional configuration.

## See Also
- [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config) for adding node groups with different instance types
- [additional_build_groups_config](/configuration/rack-parameters/aws/additional_build_groups_config) for adding dedicated build node groups
- [BuildLabels](/configuration/app-parameters/aws/BuildLabels) for directing builds to specific labeled node groups
- [Workload Placement](/configuration/scaling/workload-placement) for comprehensive placement strategies
