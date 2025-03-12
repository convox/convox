---
title: "BuildCpu"
draft: false
slug: BuildCpu
url: /reference/app-parameters/aws/BuildCpu
---

# BuildCpu

## Description
The `BuildCpu` app parameter allows you to specify the CPU request for build pods in millicores. This parameter enables you to control the amount of CPU resources allocated to build processes, allowing you to optimize build performance based on your application's build requirements.

When used in conjunction with [`BuildMem`](/reference/app-parameters/aws/BuildMem) and [`BuildLabels`](/reference/app-parameters/aws/BuildLabels), this parameter provides fine-grained control over build resource allocation and placement.

## Default Value
By default, build pods use the standard CPU allocation defined at the rack level.

## Use Cases
- **Build Optimization**: Allocate more CPU to build processes to speed up CPU-intensive builds.
- **Resource Management**: Ensure build processes have adequate CPU resources without overprovisioning.
- **Multi-tenant Clusters**: Control CPU allocation to prevent build processes from consuming excessive resources in shared environments.
- **Cost Optimization**: Allocate appropriate CPU resources based on build complexity to optimize resource usage.

## Setting the Parameter
To set the CPU request for build pods:

```html
$ convox apps params set BuildCpu=512 -a <app>
Setting BuildCpu... OK
```

This configuration allocates 512 millicores (0.5 vCPU) to build pods for the specified application.

## Viewing Current Configuration
To view the current BuildCpu setting:

```html
$ convox apps params -a <app>
NAME         VALUE
BuildCpu     512
```

## Related Parameters

### BuildMem
Sets the memory request for build pods:

```html
$ convox apps params set BuildMem=2048 -a <app>
Setting BuildMem... OK
```

### BuildLabels
Directs build pods to specific node groups:

```html
$ convox apps params set BuildLabels=convox.io/label=app-build -a <app>
Setting BuildLabels... OK
```

## Additional Information
- The `BuildCpu` value is specified in millicores, where 1000 millicores equals 1 vCPU.
- Common values include:
  - `256` (0.25 vCPU): Suitable for simple builds
  - `512` (0.5 vCPU): Balanced for most builds
  - `1024` (1 vCPU): Good for moderately complex builds
  - `2048` (2 vCPU): Recommended for CPU-intensive builds
- Setting an appropriate CPU allocation is important for build performance:
  - Too low: Builds may run slowly or time out
  - Too high: May waste resources or prevent builds from being scheduled if nodes don't have sufficient capacity
- This parameter sets a request (minimum guaranteed allocation) for CPU resources. The build pod may receive additional CPU time if available on the node.
- For optimal build performance, consider setting both `BuildCpu` and `BuildMem` appropriately based on your application's build requirements.
- When using custom build node groups with [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config), ensure the node groups have sufficient CPU capacity to accommodate your `BuildCpu` settings.

## Version Requirements
This feature is available in all recent versions of Convox.
