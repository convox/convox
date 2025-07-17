---
title: "nvidia_device_time_slicing_replicas"
draft: false
slug: nvidia_device_time_slicing_replicas
url: /configuration/rack-parameters/aws/nvidia_device_time_slicing_replicas
---

# nvidia_device_time_slicing_replicas

## Description
The `nvidia_device_time_slicing_replicas` parameter enables NVIDIA GPU time slicing by configuring the number of virtual replicas each physical GPU should be divided into. This feature allows multiple workloads to share a single physical GPU by time-sharing the GPU resources, enabling better utilization and cost optimization of expensive GPU hardware.

## Default Value
The default value for `nvidia_device_time_slicing_replicas` is not set (disabled).

## Use Cases
- **Cost Optimization**: Maximize GPU utilization by running multiple workloads on expensive GPU hardware.
- **Resource Efficiency**: Better allocation of GPU resources for workloads that don't require full GPU capacity.
- **Improved Throughput**: Support more concurrent workloads without additional hardware investment.
- **Flexible Workload Scheduling**: Enable smaller workloads like inference serving or development tasks to coexist.
- **Development and Testing**: Allow multiple developers to share GPU resources for testing and development purposes.

## Prerequisites
The NVIDIA device plugin must be enabled on your rack before using time slicing:

```html
$ convox rack params set nvidia_device_plugin_enable=true -r rackName
Setting parameters... OK
```

## Setting Parameters
To configure GPU time slicing, set the number of virtual replicas each physical GPU should be divided into:

```html
$ convox rack params set nvidia_device_time_slicing_replicas=5 -r rackName
Setting parameters... OK
```

To disable GPU time slicing:
```html
$ convox rack params unset nvidia_device_time_slicing_replicas -r rackName
Unsetting nvidia_device_time_slicing_replicas... OK
```

## Understanding Replicas
The replica count determines how many virtual GPU resources each physical GPU provides:

- `replicas=2` = Each virtual GPU gets ~50% of physical GPU capacity
- `replicas=4` = Each virtual GPU gets ~25% of physical GPU capacity  
- `replicas=5` = Each virtual GPU gets ~20% of physical GPU capacity
- `replicas=8` = Each virtual GPU gets ~12.5% of physical GPU capacity

## Verification
After applying the configuration, you can verify that nodes are advertising the correct number of GPU resources:

```html
$ kubectl describe node <gpu-node-name>
```

You should see the GPU capacity multiplied by your replica count (e.g., 1 physical GPU × 5 replicas = 5 advertised GPU resources).

## Important Considerations
- **Shared Access**: Time slicing provides shared access to GPU compute, not exclusive access to GPU resources.
- **No Memory Isolation**: There is no memory isolation between workloads sharing the same GPU. All workloads share the same GPU memory space.
- **Fault Domain**: All workloads sharing a GPU run in the same fault domain. If one workload crashes or behaves poorly, it may affect other workloads on the same GPU.
- **No Proportional Guarantees**: Requesting multiple time-sliced GPU resources does not guarantee proportional compute power or performance.
- **Performance Variability**: GPU performance may vary depending on the workload characteristics and timing of concurrent tasks.

## Best Practices
- Start with lower replica counts (2-4) and increase based on workload requirements and performance testing.
- Monitor GPU utilization and memory usage to optimize the replica count for your specific workloads.
- Consider the memory requirements of your workloads when setting replica counts, as GPU memory is shared.
- Use time slicing primarily for workloads that can tolerate performance variability and shared resources.
- Reserve dedicated GPUs for performance-critical workloads that require guaranteed resources.

## Use in convox.yml
Services can request time-sliced GPU resources using the same syntax as regular GPU resources:

```yaml
services:
  inference-service:
    build: .
    command: python inference.py
    scale:
      count: 3
      gpu: 1
```

With time slicing enabled, multiple instances of this service can share the same physical GPU.

## Related Parameters
- [nvidia_device_plugin_enable](/configuration/rack-parameters/aws/nvidia_device_plugin_enable): **Required** - Must be enabled before using time slicing.
- [gpu_tag_enable](/configuration/rack-parameters/aws/gpu_tag_enable): Enables GPU tagging for resource tracking.
- [node_type](/configuration/rack-parameters/aws/node_type): Should be set to GPU-enabled instance types (e.g., `p3.2xlarge`, `g4dn.xlarge`).

## Version Requirements
This feature requires at least Convox rack version `3.21.4`.