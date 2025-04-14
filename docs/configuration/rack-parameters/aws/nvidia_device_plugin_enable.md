---
title: "nvidia_device_plugin_enable"
draft: false
slug: nvidia_device_plugin_enable
url: /configuration/rack-parameters/aws/nvidia_device_plugin_enable
---

# nvidia_device_plugin_enable

## Description
The `nvidia_device_plugin_enable` parameter enables the NVIDIA GPU device plugin for your Kubernetes cluster. When enabled, this plugin allows Kubernetes to discover and manage NVIDIA GPUs on nodes that have them installed, making these GPUs available to your applications. The plugin deploys as a DaemonSet that runs only on GPU-capable nodes and handles the exposure of GPU resources to the Kubernetes scheduler.

## Default Value
The default value for `nvidia_device_plugin_enable` is `false`.

## Use Cases
- **Machine Learning Workloads**: Enable GPU acceleration for training and inference tasks.
- **Video Processing**: Accelerate video encoding, decoding, and transcoding operations.
- **Scientific Computing**: Support high-performance computing applications that benefit from GPU parallelization.
- **Rendering**: Enable GPU-accelerated rendering for graphics-intensive applications.
- **Deep Learning Inference**: Deploy inference engines that require GPU acceleration for optimal performance.

## Setting Parameters
To enable the NVIDIA GPU device plugin, use the following command:
```html
$ convox rack params set nvidia_device_plugin_enable=true -r rackName
Setting parameters... OK
```

To disable the NVIDIA GPU device plugin:
```html
$ convox rack params set nvidia_device_plugin_enable=false -r rackName
Setting parameters... OK
```

## Additional Information
- This parameter should only be enabled on rack instances that have NVIDIA GPUs installed. Enabling it on instances without GPUs will deploy the plugin, but it will remain inactive.
- Before enabling this parameter, ensure your AWS EC2 instances have compatible NVIDIA GPU hardware, such as instances from the P3, P4, G4, or G5 families.
- The device plugin works in conjunction with the `gpu` scaling option in your `convox.yml` file, which allows you to specify GPU requirements for your services:

```yaml
services:
  ml-service:
    build: .
    command: python train.py
    scale:
      count: 1
      gpu: 1
```

- When using GPU-enabled services, you may need to use a custom base image that includes the NVIDIA CUDA toolkit and appropriate drivers.
- GPU resources are whole units and cannot be fractionally allocated—each container requesting a GPU will receive one or more complete GPUs.
- When a service requests GPU resources, it will only be scheduled on nodes with available GPUs, which may affect scheduling and scaling behavior.

## Related Parameters
- [gpu_tag_enable](/configuration/rack-parameters/aws/gpu_tag_enable): Enables GPU tagging, which helps with identifying and tracking GPU resources in your AWS environment.
- [node_type](/configuration/rack-parameters/aws/node_type): When using GPUs, this should be set to a GPU-enabled instance type (e.g., `p3.2xlarge`, `g4dn.xlarge`).

## Version Requirements
This feature requires at least Convox rack version `3.21.0`.
