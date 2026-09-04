---
title: "build_node_min_count"
description: "The build_node_min_count AWS rack parameter sets the minimum number of build nodes to keep running, defaulting to 0 for on-demand scale-up."
slug: build_node_min_count
url: /configuration/rack-parameters/aws/build_node_min_count
---

# build_node_min_count

## Description
The `build_node_min_count` parameter specifies the minimum number of build nodes to keep running. If set to `0`, a build node will scale up when a build starts and will remain active until it has been idle for 30 minutes before scaling down.

## Default Value
The default value for `build_node_min_count` is `0`.

## Use Cases
- **Consistent Build Availability**: Ensures that there are always a minimum number of build nodes available to handle build tasks, reducing wait times and improving efficiency.
- **Performance Optimization**: Prevents delays in build processes, especially during peak development times, by maintaining a ready pool of build nodes.

## Setting Parameters
To set the `build_node_min_count` parameter, use the following command:
```bash
$ convox rack params set build_node_min_count=2 -r rackName
Updating parameters... OK
```
This command sets the minimum number of build nodes to 2.

## Additional Information
Adjusting the `build_node_min_count` allows you to manage the availability and readiness of your build infrastructure. Ensure that the value you set aligns with your team's build frequency and requirements. Keeping a higher minimum count can improve build times but will incur additional costs.

When `build_node_min_count` is set to `0`, a build node is automatically created at the start of a build and will remain active until it has been idle for 30 minutes before shutting down to conserve resources.

This parameter has no effect when [`karpenter_enabled`](/configuration/rack-parameters/aws/karpenter_enabled) is `true`. Karpenter provisions build nodes on demand through its own build NodePool and the EKS build node group is scaled to zero. Setting the value on a Karpenter Rack is accepted and changes nothing. The pre-apply raise described below is skipped when `karpenter_enabled` is `true`, so the version requirement that follows does not apply to a Karpenter Rack. See [Build Node Behavior with Karpenter](/configuration/scaling/karpenter#build-node-behavior-with-karpenter).

Raising `build_node_min_count` above the number of nodes the build node group is currently running requires a `convox` CLI at `3.25.6` or newer performing the apply. Earlier versions fail the apply with an EKS validation error and roll the value back. Setting the value at install and lowering it work on any version.

This is not a Rack version requirement. The handling lives in the `convox` binary performing the apply, which for a Console-managed Rack is the CLI bundled in the Console deploy rather than the CLI on your machine. See [Raising a node group minimum fails EKS validation](/help/troubleshooting#raising-a-node-group-minimum-fails-eks-validation) and [CLI Rack Management](/management/cli-rack-management).
