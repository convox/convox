---
title: "gpu_node_type"
description: "The gpu_node_type Oracle Cloud rack parameter sets the GPU compute shape for an optional tainted GPU node pool, unset by default."
slug: gpu_node_type
url: /configuration/rack-parameters/oci/gpu_node_type
---

# gpu_node_type

## Description
The `gpu_node_type` parameter specifies the [GPU compute shape](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm) (e.g. `VM.GPU.A10.1`) used to create a second, GPU-backed node pool for the rack. Leaving it unset (the default) skips creating a GPU node pool entirely; only the regular node pool controlled by [node_type](/configuration/rack-parameters/oci/node_type) is created.

Nodes in the GPU pool are tainted `nvidia.com/gpu=:NoSchedule`, so only pods that tolerate the taint are scheduled onto them. A service opts in by requesting a GPU with `scale.gpu` in its `convox.yml`:
```yaml
services:
  worker:
    scale:
      gpu:
        count: 1
```

## Default Value
The default value for `gpu_node_type` is `""` (no GPU node pool).

## Use Cases
- **ML/AI Workloads**: Run GPU-accelerated training or inference services alongside regular CPU services on the same rack.
- **Mixed Workloads**: Keep GPU nodes isolated via the taint so only services that explicitly request a GPU land on the more expensive GPU instances.

## Setting Parameters
To create a GPU node pool, set the shape:
```bash
$ convox rack params set gpu_node_type=VM.GPU.A10.1 -r rackName
Updating parameters... OK
```

To remove the GPU node pool, set it back to empty:
```bash
$ convox rack params set gpu_node_type="" -r rackName
Updating parameters... OK
```

## Additional Information
OCI GPU shapes are subject to tenancy service limits that default to zero in most tenancies. Before setting this parameter, request a service limit increase for the shape and region you plan to use; approval typically takes 1-3 business days. Without the increase, the GPU node pool is created but nodes fail to launch with an `OutOfHostCapacity` error.

See [gpu_node_count](/configuration/rack-parameters/oci/gpu_node_count) to size the GPU node pool.
