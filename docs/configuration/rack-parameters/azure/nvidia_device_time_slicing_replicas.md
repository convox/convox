---
title: "nvidia_device_time_slicing_replicas"
slug: nvidia_device_time_slicing_replicas
url: /configuration/rack-parameters/azure/nvidia_device_time_slicing_replicas
---

# nvidia_device_time_slicing_replicas

## Description
The `nvidia_device_time_slicing_replicas` parameter configures GPU time-slicing on the NVIDIA Device Plugin, allowing a single physical GPU to be shared across multiple pods simultaneously. When set to a value greater than `1`, the plugin advertises `N` virtual GPU units per physical GPU, enabling multiple workloads to be scheduled on the same GPU.

This requires `nvidia_device_plugin_enable` to be `true`.

## Default Value
The default value is `0` (time-slicing disabled; each GPU is advertised as a single unit).

## Use Cases
- **Development / Testing**: Share one GPU across multiple developer environments on a single node.
- **Inference Serving**: Run several lightweight inference processes concurrently on a single GPU.
- **Cost Optimisation**: Maximise GPU utilisation on expensive VM sizes like `Standard_NC`.

## Setting Parameters
```bash
$ convox rack params set nvidia_device_time_slicing_replicas=4 -r rackName
Setting parameters... OK
```

## Additional Information
Time-slicing is a software-level feature. It does not provide memory isolation between pods. Each pod can access the full GPU memory. For true isolation and guaranteed memory partitioning, use [NVIDIA MIG](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/) (available on A100 instances such as `Standard_NC24ads_A100_v4`).

Setting this to `1` is equivalent to disabling time-slicing (the default 1:1 GPU-to-resource mapping).
