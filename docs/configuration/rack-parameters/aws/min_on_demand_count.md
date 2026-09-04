---
title: "min_on_demand_count"
description: "The min_on_demand_count AWS rack parameter sets the minimum number of on-demand nodes when using the mixed node capacity type, defaulting to 1."
slug: min_on_demand_count
url: /configuration/rack-parameters/aws/min_on_demand_count
---

# min_on_demand_count

## Description
The `min_on_demand_count` parameter sets the minimum number of on-demand nodes when using the `mixed` node capacity type. This allows you to ensure a baseline of on-demand instances in your cluster.

## Default Value
The default value for `min_on_demand_count` is `1`.

## Use Cases
- **Reliability**: Ensure that a minimum number of reliable on-demand instances are always available to handle workloads.
- **Performance Assurance**: Maintain a specific number of on-demand nodes to meet performance and reliability requirements.

## Setting Parameters
To set the `min_on_demand_count` parameter, use the following command:
```bash
$ convox rack params set min_on_demand_count=2 -r rackName
Updating parameters... OK
```
This command sets the minimum number of on-demand nodes to 2.

## Additional Information
The `min_on_demand_count` parameter is used in conjunction with the [node_capacity_type](/configuration/rack-parameters/aws/node_capacity_type) parameter. When the `node_capacity_type` is set to `mixed`, you can specify the minimum and maximum number of on-demand nodes to balance cost and availability.

Adjusting the `min_on_demand_count` helps you ensure that there are always a sufficient number of reliable on-demand nodes available for your workloads, complementing the use of spot instances to reduce costs.

Additionally, consider configuring the [max_on_demand_count](/configuration/rack-parameters/aws/max_on_demand_count) parameter to limit the maximum number of on-demand nodes and optimize resource allocation.

This parameter has no effect when [`karpenter_enabled`](/configuration/rack-parameters/aws/karpenter_enabled) is `true`, because Karpenter holds every system node group at one node. Enabling Karpenter also requires `node_capacity_type=ON_DEMAND`, so a Karpenter Rack is not a `mixed` Rack and the two settings do not normally meet. See [Karpenter](/configuration/scaling/karpenter#enablement-validation-guards).

On AWS Racks, raising `min_on_demand_count` above the number of nodes the on-demand node group is currently running requires a `convox` CLI at `3.25.6` or newer performing the apply. Earlier versions fail the apply with an EKS validation error and roll the value back. Setting the value at install and lowering it work on any version.

This is not a Rack version requirement. The handling lives in the `convox` binary performing the apply, which for a Console-managed Rack is the CLI bundled in the Console deploy rather than the CLI on your machine. See [Raising a node group minimum fails EKS validation](/help/troubleshooting#raising-a-node-group-minimum-fails-eks-validation) and [CLI Rack Management](/management/cli-rack-management).
