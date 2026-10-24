---
title: "node_type"
description: "The node_type GCP rack parameter sets the x86 Compute Engine machine type used for rack nodes, defaulting to n1-standard-2."
slug: node_type
url: /configuration/rack-parameters/gcp/node_type
---

# node_type

## Description
The `node_type` parameter specifies the type of instance to use for nodes in your Convox rack. This allows you to choose the appropriate instance type based on your application's requirements.

## Default Value
The default value for `node_type` is `n1-standard-2`.

## Use Cases
- **Performance Optimization**: Select an instance type that provides the necessary CPU, memory, and network performance for your application.
- **Cost Management**: Choose an instance type that balances cost with the required performance characteristics.

## Setting Parameters
To set the `node_type` parameter, use the following command:
```bash
$ convox rack params set node_type=n1-standard-4 -r rackName
Updating parameters... OK
```
This command sets the `node_type` parameter to the specified value.

## CPU Architecture

`node_type` must be an x86 machine type. GKE taints Arm nodes, and some Rack services run only on x86 nodes, so the primary node pool stays x86. Arm machine types (T2A, C4A, N4A and A4X) run as additional node pools; see [Arm Node Pools](/configuration/rack-parameters/gcp/additional_node_groups_config#arm-node-pools).

With CLI 3.25.12 or later, `convox rack params set` refuses an Arm machine type:

```bash
$ convox rack params set node_type=t2a-standard-4 -r rackName
Updating parameters... ERROR: param 'node_type' t2a-standard-4 is an Arm machine type; the primary node pool must stay x86
  Add Arm nodes with additional_node_groups_config instead.
  Re-run with --force to proceed
```

An Arm primary node pool is not supported, so do not re-run the command with `--force`. The Console refuses an Arm `node_type` when it installs a Rack, but `convox rack install` does not check it, so choose an x86 machine type when you install a Rack from the CLI.

## Additional Information
Selecting the appropriate `node_type` is crucial for ensuring that your applications run efficiently and cost-effectively. Consider the specific needs of your workload when choosing an instance type. For more information on GCP instance types, refer to the [GCP documentation](https://cloud.google.com/compute/docs/machine-resource).
