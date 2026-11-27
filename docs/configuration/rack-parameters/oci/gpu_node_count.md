---
title: "gpu_node_count"
description: "The gpu_node_count Oracle Cloud rack parameter sets the number of nodes in the optional GPU node pool, defaulting to 1."
slug: gpu_node_count
url: /configuration/rack-parameters/oci/gpu_node_count
---

# gpu_node_count

## Description
The `gpu_node_count` parameter specifies how many nodes run in the rack's GPU node pool. It only takes effect when [gpu_node_type](/configuration/rack-parameters/oci/gpu_node_type) is set; with no GPU shape configured, no GPU node pool exists and this parameter has no effect.

## Default Value
The default value for `gpu_node_count` is `1`.

## Use Cases
- **GPU Capacity Planning**: Add GPU nodes to run more GPU-scheduled Processes concurrently.
- **Cost Management**: Keep the GPU pool at a single node, or scale it to zero by clearing [gpu_node_type](/configuration/rack-parameters/oci/gpu_node_type), when GPU capacity is not in active use.

## Setting Parameters
To set the `gpu_node_count` parameter, use the following command:
```bash
$ convox rack params set gpu_node_count=2 -r rackName
Updating parameters... OK
```
This command sets the GPU node pool to 2 nodes.

## Additional Information
Each GPU node is subject to the same tenancy service limits as the shape set in [gpu_node_type](/configuration/rack-parameters/oci/gpu_node_type); increasing this count may require a larger service limit increase.
