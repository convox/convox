---
title: "node_count"
description: "The node_count Oracle Cloud rack parameter sets the number of nodes in the rack's node pool, defaulting to 2."
slug: node_count
url: /configuration/rack-parameters/oci/node_count
---

# node_count

## Description
The `node_count` parameter specifies how many nodes run in the rack's node pool. OKE node pools are fixed size, so this parameter is the only way to change how many nodes the pool runs; there is no built-in autoscaler.

## Default Value
The default value for `node_count` is `2`.

## Use Cases
- **Capacity Planning**: Add nodes to handle more services or higher load.
- **Cost Management**: Reduce the node count for smaller workloads or non-production racks.

## Setting Parameters
To set the `node_count` parameter, use the following command:
```bash
$ convox rack params set node_count=4 -r rackName
Updating parameters... OK
```
This command sets the node pool to 4 nodes.

## Additional Information
Nodes are spread evenly across the availability domains in the region, regardless of [high_availability](/configuration/rack-parameters/oci/high_availability). This parameter only affects the primary node pool; see [gpu_node_count](/configuration/rack-parameters/oci/gpu_node_count) for the optional GPU node pool.
