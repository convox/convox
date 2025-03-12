---
title: "BuildMem"
draft: false
slug: BuildMem
url: /reference/app-parameters/aws/BuildMem
---

# BuildMem

## Description
The `BuildMem` app parameter allows you to specify the memory request for build pods in megabytes. This parameter enables you to control the amount of memory resources allocated to build processes, helping you optimize build performance and prevent out-of-memory errors during complex builds.

When used in conjunction with [`BuildCpu`](/reference/app-parameters/aws/BuildCpu) and [`BuildLabels`](/reference/app-parameters/aws/BuildLabels), this parameter provides comprehensive control over build resource allocation and placement.

## Default Value
By default, build pods use the standard memory allocation defined at the rack level.

## Use Cases
- **Memory-Intensive Builds**: Allocate more memory for builds involving large dependencies, asset compilation, or memory-intensive operations.
- **Out-of-Memory Prevention**: Prevent build failures caused by insufficient memory allocation.
- **Resource Management**: Ensure build processes have adequate memory resources without overprovisioning.
- **Docker Build Optimization**: Provide sufficient memory for Docker layer caching and image building operations.

## Setting the Parameter
To set the memory request for build pods:

```html
$ convox apps params set BuildMem=2048 -a <app>
Setting BuildMem... OK
```

This configuration allocates 2048MB (2GB) of memory to build pods for the specified application.

## Viewing Current Configuration
To view the current BuildMem setting:

```html
$ convox apps params -a <app>
NAME         VALUE
BuildMem     2048
```

## Related Parameters

### BuildCpu
Sets the CPU request for build pods:

```html
$ convox apps params set BuildCpu=512 -a <app>
Setting BuildCpu... OK
```

### BuildLabels
Directs build pods to specific node groups:

```html
$ convox apps params set BuildLabels=convox.io/label=app-build -a <app>
Setting BuildLabels... OK
```

## Additional Information
- The `BuildMem` value is specified in megabytes (MB).
- Common values include:
  - `512`: Suitable for simple builds
  - `1024` (1GB): Balanced for most builds
  - `2048` (2GB): Good for builds with moderate dependencies
  - `4096` (4GB): Recommended for memory-intensive builds
  - `8192` (8GB): For very large or complex builds
- Setting an appropriate memory allocation is critical for successful builds:
  - Too low: Builds may fail with out-of-memory errors
  - Too high: May waste resources or prevent builds from being scheduled if nodes don't have sufficient capacity
- This parameter sets a request (minimum guaranteed allocation) for memory resources. Unlike CPU, memory is a hard limit, and exceeding it will cause the build process to be terminated.
- Common symptoms of insufficient build memory include:
  - Build failures during dependency installation
  - Node.js or Yarn reporting heap out-of-memory errors
  - Compilation or asset processing failures
  - Docker build errors during large image creation
- For optimal build performance, consider setting both `BuildMem` and `BuildCpu` appropriately based on your application's build requirements.
- When using custom build node groups with [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config), ensure the node groups have sufficient memory capacity to accommodate your `BuildMem` settings.

## Version Requirements
This feature is available in all recent versions of Convox.
