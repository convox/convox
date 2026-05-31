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

## Choosing an Autoscaler

Convox offers several ways to set the size of a service. Start here to pick the right one, then jump to the matching section below.

- **`scale.autoscale` (recommended):** Preconfigured KEDA-based triggers for CPU, memory, GPU utilization, and queue depth, including scale-to-zero. Use this when you want event-driven or utilization-driven autoscaling with minimal configuration. Requires `keda_enable=true` on the rack.
- **KEDA (raw triggers):** Drop down to raw KEDA ScaleTriggers when you need a scaler outside the four built-in types (SQS, CloudWatch, Datadog, cron, and 60+ other sources). See [KEDA Autoscaling](/configuration/scaling/keda).
- **Manual replica counts:** Set a fixed `count` and adjust it by hand with `convox scale`. Use this when traffic is steady or predictable and you do not want automatic adjustment.
- **Horizontal Autoscaling (HPA), legacy:** The `scale.targets` block uses native Kubernetes HPA and does not require KEDA. Prefer `scale.autoscale` for new services; use `scale.targets` if you cannot enable KEDA on the rack.

## Event-Driven Autoscaling (scale.autoscale)

The `scale.autoscale` block provides preconfigured KEDA-based autoscaling triggers with minimal configuration. Instead of writing raw KEDA trigger definitions, you specify a trigger type and a threshold value. Convox handles the KEDA ScaledObject configuration, Prometheus queries, and activation thresholds automatically.

> Requires `keda_enable=true` on the rack. See [keda_enable](/configuration/rack-parameters/aws/keda_enable).

### Available Triggers

| Trigger | Signal | Use case |
|---------|--------|----------|
| `cpu` | CPU utilization % | Web services, API servers |
| `memory` | Memory utilization % | Cache-heavy services, data processing |
| `gpuUtilization` | GPU utilization % via DCGM | ML inference, GPU-accelerated workloads |
| `queueDepth` | Prometheus metric value | Inference request queues, job queues |

### CPU Autoscaling

Scale a web service between 2 and 10 replicas based on CPU utilization:

```yaml
services:
  web:
    build: .
    port: 3000
    scale:
      min: 2
      max: 10
      autoscale:
        cpu:
          threshold: 70
```

### Scale to Zero

Scale a worker to zero when idle, spinning up automatically when CPU load increases:

```yaml
services:
  worker:
    build: .
    command: bin/process
    scale:
      min: 0
      max: 5
      autoscale:
        cpu:
          threshold: 50
```

Services at zero replicas show a `COLD` status indicator in `convox scale` output. The first request or trigger activation incurs a cold-start delay while a replica provisions.

### GPU Inference Autoscaling

Scale a GPU inference service based on GPU utilization, with scale-to-zero when no requests are arriving:

```yaml
services:
  vllm:
    build: .
    port: 8000
    scale:
      min: 0
      max: 10
      gpu:
        count: 1
        vendor: nvidia
      autoscale:
        gpuUtilization:
          threshold: 70
```

> Requires `gpu_observability_enable=true` and `prometheus_url` set on the rack.

### Queue Depth Autoscaling

Scale based on inference request queue depth (or any Prometheus metric):

```yaml
services:
  worker:
    build: .
    scale:
      min: 0
      max: 5
      autoscale:
        queueDepth:
          threshold: 5
          metricName: vllm:num_requests_waiting
```

### Combined Triggers

Multiple triggers can be combined. KEDA scales to whichever trigger demands the most replicas:

```yaml
services:
  inference:
    build: .
    port: 8000
    scale:
      min: 1
      max: 8
      gpu:
        count: 1
        vendor: nvidia
      autoscale:
        cpu:
          threshold: 70
        gpuUtilization:
          threshold: 75
        queueDepth:
          threshold: 3
        cooldownPeriod: 300
        pollingInterval: 15
```

### autoscale Reference

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| **cpu** | map | | CPU utilization trigger. Sub-key: `threshold` (1-100, percent) |
| **memory** | map | | Memory utilization trigger. Sub-key: `threshold` (1-100, percent) |
| **gpuUtilization** | map | | GPU utilization trigger via Prometheus/DCGM. Sub-keys: `threshold` (1-100), optional `metricName`, `prometheusUrl`, `query` |
| **queueDepth** | map | | Queue depth trigger via Prometheus. Sub-keys: `threshold` (> 0), optional `metricName` (default: `vllm:num_requests_waiting`), `prometheusUrl`, `query` |
| **custom** | list | | Raw KEDA ScaleTriggers for advanced use cases beyond the four built-in types |
| **cooldownPeriod** | number | 300 | Seconds to wait after the last trigger activation before scaling down |
| **pollingInterval** | number | 30 | Seconds between trigger checks |

For raw KEDA trigger configuration (SQS, CloudWatch, Datadog, cron, and 60+ other scalers), see [KEDA Autoscaling](/configuration/scaling/keda).

## Manual Scaling

### Determine Current Scale
```bash
    $ convox scale
    SERVICE  DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  STATUS
    web      2        2        250  512     -    -    -
```

> Columns 1-6 (`SERVICE`, `DESIRED`, `RUNNING`, `CPU`, `MEMORY`, `GPU`) match the 3.24.5 layout exactly. 3.24.6 appends `MIN`, `MAX`, an optional `AUTOSCALE`, and a trailing `STATUS` column. See [`convox scale` output reference](/reference/cli/scale#output-table) for the full column-position contract.
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

> For most use cases, the `scale.autoscale` block above is the recommended approach. The `scale.targets` block below uses native Kubernetes HPA and does not require KEDA.

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

- **Restrictive PodDisruptionBudgets (PDBs)**: A PDB with `minAvailable: 1` on a service with one replica prevents that healthy pod from being evicted. Adjust with the [`pdb_default_min_available_percentage`](/configuration/rack-parameters/aws/pdb_default_min_available_percentage) rack parameter. Unhealthy pods (CrashLoopBackOff, Error) do not block eviction. Convox PDBs use `unhealthyPodEvictionPolicy: AlwaysAllow` so that stuck pods cannot prevent node scale-down. To opt a specific service out of the Convox-managed PDB entirely, set the `convox.com/pdb-disabled=true` annotation on the service (see [Disabling PDB for a Service](#disabling-pdb-for-a-service) below).
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

### Disabling PDB for a Service

Convox creates a PodDisruptionBudget for each service by default. To opt a specific service out, add the `convox.com/pdb-disabled=true` annotation:

```yaml
services:
  web:
    build: .
    port: 3000
    annotations:
      - convox.com/pdb-disabled=true
```

With PDB disabled, the service's pods can be evicted without budget protection during node scale-down, node drain, or maintenance events. Use only on services that tolerate unplanned disruption, for example stateless workers that can be restarted anywhere at any time.

Both `convox.com/pdb-disabled` (canonical) and `convox.com/pdb-disbaled` (legacy spelling, kept for backward compatibility) are accepted. New configurations should use the canonical spelling.

## See Also

- [convox.yml](/configuration/convox-yml) for configuring scale defaults
- [VPA](/configuration/scaling/vpa) for automatic resource right-sizing
- [KEDA Autoscaling](/configuration/scaling/keda) for event-driven autoscaling
- [Datadog Metrics](/configuration/scaling/datadog-metrics) for Datadog-based autoscaling
- [Karpenter](/configuration/scaling/karpenter) for pod-level node provisioning as an alternative to Cluster Autoscaler (AWS only)
- [Console Autoscale Triggers](/console/autoscale-triggers)
