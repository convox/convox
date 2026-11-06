---
title: "node_memory"
description: "The node_memory Oracle Cloud rack parameter sets the memory in GB per node for Flex compute shapes, defaulting to 16."
slug: node_memory
url: /configuration/rack-parameters/oci/node_memory
---

# node_memory

## Description
The `node_memory` parameter specifies the amount of memory, in GB, allocated to each node in the rack's node pool. It only takes effect when [node_type](/configuration/rack-parameters/oci/node_type) is a Flex shape (a shape name ending in `.Flex`, such as the default `VM.Standard.E4.Flex`); fixed shapes have a set memory size that this parameter cannot change.

## Default Value
The default value for `node_memory` is `16`.

## Use Cases
- **Memory-Bound Workloads**: Increase memory for services that cache heavily or run memory-intensive processes.
- **Cost Management**: Reduce memory on a Flex shape to right-size nodes for lighter workloads.

## Setting Parameters
To set the `node_memory` parameter, use the following command:
```bash
$ convox rack params set node_memory=32 -r rackName
Updating parameters... OK
```
This command sets the node memory allocation to 32GB.

## Additional Information
Set alongside [node_ocpus](/configuration/rack-parameters/oci/node_ocpus) to size Flex shape nodes independently for CPU and memory. Changing this parameter replaces the rack's nodes.
