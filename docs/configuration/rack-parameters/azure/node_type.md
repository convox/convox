---
title: "node_type"
description: "The node_type Azure rack parameter sets the VM SKU used for rack nodes, requiring an x86 SKU, defaulting to Standard_D2_v3."
slug: node_type
url: /configuration/rack-parameters/azure/node_type
---

# node_type

## Description
The `node_type` parameter specifies the type of instance to use for nodes in your Convox rack. This allows you to choose the appropriate instance type based on your application's requirements.

## Default Value
The default value for `node_type` is `Standard_D2_v3`.

## Use Cases
- **Performance Optimization**: Select an instance type that provides the necessary CPU, memory, and network performance for your application.
- **Cost Management**: Choose an instance type that balances cost with the required performance characteristics.

## Setting Parameters
To set the `node_type` parameter, use the following command:
```bash
$ convox rack params set node_type=Standard_D2_v3 -r rackName
Updating parameters... OK
```
This command sets the `node_type` parameter to the specified value.

Changing `node_type` replaces every node of the default node pool through a temporary node pool named `defaulttemp`. Pods on the default pool are drained and rescheduled twice, onto the temporary pool and then back onto the new nodes.

## CPU Architecture

`node_type` must be an x86 VM size. Some Rack services run only on x86 nodes, so the default node pool stays x86. Arm VM sizes, which have a `p` after the vCPU count, such as `Standard_D4ps_v5` or `Standard_E4pds_v5`, run as additional node pools; see [Arm Node Pools](/configuration/rack-parameters/azure/additional_node_groups_config#arm-node-pools).

With CLI 3.25.12 or later, `convox rack params set` refuses an Arm VM size:

```bash
$ convox rack params set node_type=Standard_D4ps_v5 -r rackName
Updating parameters... ERROR: param 'node_type' Standard_D4ps_v5 is an Arm machine type; the primary node pool must stay x86
  Add Arm nodes with additional_node_groups_config instead.
  Re-run with --force to proceed
```

An Arm default node pool is not supported, so do not re-run the command with `--force`. The Console refuses an Arm `node_type` when it installs a Rack, but `convox rack install` does not check it, so choose an x86 VM size when you install a Rack from the CLI.

## Additional Information
Selecting the appropriate `node_type` is crucial for ensuring that your applications run efficiently and cost-effectively. Consider the specific needs of your workload when choosing an instance type. For more information on Azure instance types, refer to the [Azure documentation](https://azure.microsoft.com/en-us/pricing/details/virtual-machines/series/).
