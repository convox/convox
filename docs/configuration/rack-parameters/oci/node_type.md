---
title: "node_type"
description: "The node_type Oracle Cloud rack parameter sets the compute shape used for rack nodes, defaulting to VM.Standard.E4.Flex."
slug: node_type
url: /configuration/rack-parameters/oci/node_type
---

# node_type

## Description
The `node_type` parameter specifies the [compute shape](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm) to use for nodes in your Convox rack's node pool.

## Default Value
The default value for `node_type` is `VM.Standard.E4.Flex`.

## Use Cases
- **Performance Optimization**: Select a shape that provides the necessary CPU, memory, and network performance for your application.
- **Cost Management**: Choose a shape that balances cost with the required performance characteristics.

## Setting Parameters
To set the `node_type` parameter, use the following command:
```bash
$ convox rack params set node_type=VM.Standard.E4.Flex -r rackName
Updating parameters... OK
```
This command sets the `node_type` parameter to the specified value.

## Additional Information
Flex shapes, such as the default `VM.Standard.E4.Flex`, let you size CPU and memory independently with [node_ocpus](/configuration/rack-parameters/oci/node_ocpus) and [node_memory](/configuration/rack-parameters/oci/node_memory); those two parameters have no effect on fixed (non-Flex) shapes. For more information on OCI compute shapes, refer to the [OCI documentation](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm).
