---
title: "nvidia_device_plugin_enable"
slug: nvidia_device_plugin_enable
url: /configuration/rack-parameters/azure/nvidia_device_plugin_enable
---

# nvidia_device_plugin_enable

## Description
The `nvidia_device_plugin_enable` parameter deploys the [NVIDIA GPU Device Plugin](https://github.com/NVIDIA/k8s-device-plugin) DaemonSet to your AKS cluster. This plugin exposes `nvidia.com/gpu` as a schedulable Kubernetes resource, allowing workloads to request GPU access.

When using Azure GPU VM sizes (e.g. `Standard_NC`, `Standard_ND`, `Standard_NV` families), AKS automatically installs the NVIDIA kernel drivers on the nodes. This parameter adds the device plugin layer on top, which is required for Kubernetes to advertise and schedule GPU resources.

The plugin is pinned to GPU-capable nodes using the `convox.io/gpu-vendor=nvidia` node label, which Convox automatically applies when it detects a GPU VM size.

## Default Value
The default value for `nvidia_device_plugin_enable` is `false`.

## Use Cases
- **Machine Learning**: Enable GPU acceleration for model training and inference.
- **Video Processing**: Accelerate encoding/decoding workloads with NVIDIA hardware.
- **Scientific Computing**: Run CUDA-accelerated HPC applications.
- **Deep Learning Inference**: Deploy GPU-backed inference servers (e.g. Triton, TorchServe).

## Setting Parameters
This parameter must be set at rack installation time:
```bash
$ convox install azure --params nvidia_device_plugin_enable=true
```

Or updated after installation:
```bash
$ convox rack params set nvidia_device_plugin_enable=true -r rackName
Setting parameters... OK
```

## Additional Information
GPU workloads in your `convox.yml` request GPUs via the `gpu` service field. The node pool running GPU VMs should use an `NC*`, `ND*`, or `NV*` VM size (via `node_type` or `additional_node_groups_config`). See also `nvidia_device_time_slicing_replicas` for sharing a single GPU across multiple workloads.
