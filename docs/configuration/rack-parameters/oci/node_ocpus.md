---
title: "node_ocpus"
description: "The node_ocpus Oracle Cloud rack parameter sets the number of OCPUs per node for Flex compute shapes, defaulting to 2."
slug: node_ocpus
url: /configuration/rack-parameters/oci/node_ocpus
---

# node_ocpus

## Description
The `node_ocpus` parameter specifies the number of OCPUs allocated to each node in the rack's node pool. It only takes effect when [node_type](/configuration/rack-parameters/oci/node_type) is a Flex shape (a shape name ending in `.Flex`, such as the default `VM.Standard.E4.Flex`); fixed shapes have a set OCPU count that this parameter cannot change.

## Default Value
The default value for `node_ocpus` is `2`.

## Use Cases
- **Compute-Bound Workloads**: Increase OCPUs for services that are CPU-limited rather than memory-limited.
- **Cost Management**: Reduce OCPUs on a Flex shape to right-size nodes for lighter workloads.

## Setting Parameters
To set the `node_ocpus` parameter, use the following command:
```bash
$ convox rack params set node_ocpus=4 -r rackName
Updating parameters... OK
```
This command sets the node OCPU count to 4.

## Additional Information
Set alongside [node_memory](/configuration/rack-parameters/oci/node_memory) to size Flex shape nodes independently for CPU and memory. Changing this parameter replaces the rack's nodes.
