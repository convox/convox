---
title: "AWS App Parameters"
draft: false
slug: aws-app-parameters
url: /configuration/app-parameters/aws
---
# AWS App Parameters

The following parameters are available for configuring your Convox applications on Amazon Web Services (AWS). These parameters allow you to customize and optimize the behavior of your applications running on the AWS platform.

## Parameters

| Parameter | Description |
|:---------|:------------|
| [BuildLabels](/configuration/app-parameters/aws/BuildLabels) | Specifies Kubernetes node selector labels for build pods |
| [BuildCpu](/configuration/app-parameters/aws/BuildCpu) | Sets the CPU request for build pods in millicores |
| [BuildMem](/configuration/app-parameters/aws/BuildMem) | Sets the memory request for build pods in megabytes |

> **Warning**: When configuring `BuildLabels`, ensure that the specified labels match those defined in the [`additional_build_groups`](/configuration/rack-parameters/aws/additional_build_groups) rack parameter. If the labels don't match any existing build node groups, build pods will remain in a pending state indefinitely, preventing builds from completing.

These application-specific parameters complement the rack-level configuration available through [Rack Parameters](/configuration/rack-parameters/aws), providing fine-grained control over your application deployments.

## Setting Parameters

To set an app parameter, use the following command:
```html
$ convox apps params set parameterName=value -a appName
Setting parameters... OK
```

For example, to set the `BuildLabels` parameter:
```html
$ convox apps params set BuildLabels=convox.io/label=app-build -a myapp
Setting BuildLabels... OK
```

## Viewing Parameters

To view the current parameters for an application:
```html
$ convox apps params -a appName
NAME         VALUE
BuildLabels  convox.io/label=app-build
BuildCpu     512
BuildMem     1024
```
