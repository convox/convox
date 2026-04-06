---
title: "Autoscaling"
slug: autoscaling
url: /configuration/scaling/autoscaling
---
# Autoscaling

Convox allows you to scale any [Service](/reference/primitives/app/service) on the following dimensions:

- Horizontal concurrency (number of [Processes](/reference/primitives/app/process))
- CPU allocation (in CPU units where 1000 units is one full CPU)
- Memory allocation (in MB)
- GPU allocation (number of GPUs per process)

## Initial Defaults

You can specify the scale for any [Service](/reference/primitives/app/service) in your [convox.yml](/configuration/convox-yml)
```yaml
services:
  web:
    scale:
      count: 2
      cpu: 250
      memory: 512
```
> If you specify a static `count` it will only be used on first deploy. Subsequent changes must be made using the `convox` CLI.

For GPU-accelerated workloads, you can specify the number of GPUs required:
```yaml
services:
  ml-worker:
    scale:
      count: 1
      cpu: 1000
      memory: 2048
      gpu: 1
```

## Manual Scaling

### Determine Current Scale
```bash
    $ convox scale
    SERVICE  DESIRED  RUNNING  CPU  MEMORY
    web      2        2        250  512
```
### Scaling Count Horizontally
```bash
    $ convox scale web --count=3
    Scaling web...
    2026-01-15T14:30:00Z system/k8s/web Scaled up replica set web-65f45567d to 2
    2026-01-15T14:30:00Z system/k8s/web-65f45567d Created pod: web-65f45567d-c7sdw
    2026-01-15T14:30:00Z system/k8s/web-65f45567d-c7sdw Successfully assigned dev-convox/web-65f45567d-c7sdw to node
    2026-01-15T14:30:00Z system/k8s/web-65f45567d-c7sdw Container image "registry.dev.convox/convox:web.BABCDEFGHI" already present on machine
    2026-01-15T14:30:01Z system/k8s/web-65f45567d-c7sdw Created container main
    2026-01-15T14:30:01Z system/k8s/web-65f45567d-c7sdw Started container main
    OK
```
> Changes to `cpu`, `memory`, or `gpu` should be done in your `convox.yml`, and a new release of your app deployed.

## Horizontal Autoscaling (HPA)

To use autoscaling you must specify a range for allowable [Process](/reference/primitives/app/process) count and
target values for CPU and Memory utilization (in percent):
```yaml
services:
  web:
    scale:
      count: 1-10
      targets:
        cpu: 70
        memory: 90
```
The number of [Processes](/reference/primitives/app/process) will be continually adjusted to maintain your target metrics.

You must consider that the targets for CPU and Memory use the service replicas limits to calculate the utilization percentage. So if you set the target for CPU as `70` and have two replicas, it will trigger the auto-scale only if the utilization percentage sum divided by the replica's count is bigger than 70%. The desired replicas will be calculated to satisfy the percentage. Being the `currentMetricValue` computed by taking the average of the given metric across all service replicas.

```text
desiredReplicas = ceil[currentReplicas * ( currentMetricValue / desiredMetricValue )]
```

## GPU Scaling

For workloads that require GPU acceleration, Convox supports requesting GPU resources at the service level. This is particularly useful for machine learning, video processing, and scientific computing applications.

### Prerequisites for GPU Scaling

Before using GPU scaling:

1. Your rack must be running on GPU-capable instances:
   - **AWS**: EC2 p3, p4, g4, or g5 instance families
   - **Azure**: NC, ND, or NV series virtual machines
2. The NVIDIA device plugin must be enabled on your rack:
```bash
$ convox rack params set nvidia_device_plugin_enable=true -r rackName
```
See the NVIDIA device plugin rack parameter for your provider: [AWS](/configuration/rack-parameters/aws/nvidia_device_plugin_enable) | [Azure](/configuration/rack-parameters/azure/nvidia_device_plugin_enable).

### Configuring GPU Requirements

You can specify GPU requirements in the `scale` section of your service definition:

```yaml
services:
  ml-trainer:
    build: .
    command: python train.py
    scale:
      count: 1-3
      cpu: 1000
      memory: 4096
      gpu: 1
```

This configuration requests 1 GPU for each process of the `ml-trainer` service.

You can also specify the GPU vendor using the map form:

```yaml
services:
  ml-trainer:
    build: .
    command: python train.py
    scale:
      count: 1-3
      cpu: 1000
      memory: 4096
      gpu:
        count: 1
        vendor: nvidia
```

See the [Service scale.gpu](/reference/primitives/app/service#scalegpu) reference for the full GPU configuration options.

### Important Notes About GPU Scaling

- GPUs are allocated as whole units (you cannot request a fraction of a GPU)
- Services requesting GPUs will only be scheduled on nodes that have available GPUs
- Each process will receive the specified number of GPUs
- If you specify a GPU count without specifying CPU or memory resources, the defaults for those resources will be removed to allow for pure GPU-based scheduling
- When using GPUs, you may need to use a base image that includes the NVIDIA CUDA toolkit

### Combining GPU with Autoscaling

GPU-enabled services can be configured with autoscaling:

```yaml
services:
  ml-inference:
    build: .
    command: python serve_model.py
    scale:
      count: 1-5
      cpu: 1000
      memory: 2048
      gpu: 1
      targets:
        cpu: 80
```

The service will scale based on CPU utilization while ensuring that each process has access to a GPU.

## Troubleshooting Cluster Scale-Down

> If you are using [Karpenter](/configuration/scaling/karpenter) for node provisioning, Karpenter handles node consolidation automatically based on `karpenter_consolidation_enabled` and `karpenter_consolidate_after`. The Cluster Autoscaler troubleshooting below applies only to Racks using the default Cluster Autoscaler.

If your cluster is not scaling down despite low resource usage, the Kubernetes Cluster Autoscaler may be blocked from removing nodes. Common causes:

- **Restrictive PodDisruptionBudgets (PDBs)**: A PDB with `minAvailable: 1` on a service with one replica prevents that pod from being evicted. Adjust with the [`pdb_default_min_available_percentage`](/configuration/rack-parameters/aws/pdb_default_min_available_percentage) rack parameter.
- **System pods**: Pods in the `kube-system` namespace may have rules preventing eviction.
- **Pods without a controller**: Pods not managed by a Deployment or ReplicaSet will not be evicted.
- **Pods with local storage**: Pods using `hostPath` or `emptyDir` volumes cannot be moved.
- **Scheduling constraints**: Node selectors or anti-affinity rules may prevent rescheduling onto other nodes.

To diagnose, inspect the Cluster Autoscaler logs:

```bash
$ kubectl logs -n kube-system deployment/cluster-autoscaler
```

Look for messages like `pod <namespace>/<pod_name> is blocking scale down`. You can also check for restrictive PDBs:

```bash
$ kubectl get pdb -A
```

A PDB with `ALLOWED DISRUPTIONS` of `0` will block evictions on that node.

## See Also

- [convox.yml](/configuration/convox-yml) for configuring scale defaults
- [VPA](/configuration/scaling/vpa) for automatic resource right-sizing
- [KEDA Autoscaling](/configuration/scaling/keda) for event-driven autoscaling
- [Datadog Metrics](/configuration/scaling/datadog-metrics) for Datadog-based autoscaling
- [Karpenter](/configuration/scaling/karpenter) for pod-level node provisioning as an alternative to Cluster Autoscaler (AWS only)
