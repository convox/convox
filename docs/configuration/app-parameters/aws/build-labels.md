---
title: "BuildLabels"
draft: false
slug: BuildLabels
url: /reference/app-parameters/aws/BuildLabels
---

# BuildLabels

## Description
The `BuildLabels` app parameter allows you to specify Kubernetes node selector labels for build pods. This parameter enables you to direct build processes to specific node groups within your cluster, providing control over where builds are executed.

When used in conjunction with [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config) or [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config) rack parameters, this feature enables fine-grained control over build workload placement.

## Default Value
By default, no build labels are set, and build pods are scheduled according to standard Kubernetes scheduling rules.

## Use Cases
- **Build Isolation**: Direct build pods to dedicated nodes to prevent resource contention with production services.
- **Resource Optimization**: Target build processes to nodes with higher CPU or memory capacities to speed up build times.
- **Cost Management**: Route builds to spot instance node groups for cost savings on non-critical, interruptible build workloads.
- **Specialized Hardware**: Direct builds to nodes with specific hardware profiles (e.g., faster disk I/O) for improved build performance.

## Setting the Parameter
To set build node selector labels for an application:

```html
$ convox apps params set BuildLabels=convox.io/label=app-build -a <app-name>
Setting BuildLabels... OK
```

This configuration directs build pods for the specified application to nodes with the label `convox.io/label: app-build`.

### Using Multiple Labels
You can specify multiple labels using a comma-separated list:

```html
$ convox apps params set BuildLabels=convox.io/label=app-build,build-type=large -a <app>
Setting BuildLabels... OK
```

## Viewing Current Configuration
To view the current build labels for an application:

```html
$ convox apps params -a <app>
NAME         VALUE
BuildLabels  convox.io/label=app-build
```

## Removing Build Labels
To remove build labels:

```html
$ convox apps params unset BuildLabels -a <app>
Unsetting BuildLabels... OK
```

## Related Parameters

### BuildCpu
Sets the CPU request for build pods in millicores:

```html
$ convox apps params set BuildCpu=512 -a <app>
Setting BuildCpu... OK
```

This allocates 512 millicores (0.5 vCPU) to build pods.

### BuildMem
Sets the memory request for build pods in megabytes:

```html
$ convox apps params set BuildMem=2048 -a <app>
Setting BuildMem... OK
```

This allocates 2048MB (2GB) of memory to build pods.

## Additional Information
The `BuildLabels` parameter works by adding node selector constraints to Kubernetes build pods. This ensures that pods are only scheduled on nodes with matching labels.

For this to be effective, you need to:

1. Configure node groups with appropriate labels using rack parameters like [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config).
2. Set `BuildLabels` to match those node labels.

Specifying incorrect labels that don't match any existing nodes in your cluster can cause build failures, as Kubernetes won't be able to schedule the build pods. Always verify that the labels you specify match labels that exist on your cluster nodes.

For more information on node selection and workload placement strategies, see the [Workload Placement](/configuration/workload-placement) guide.
