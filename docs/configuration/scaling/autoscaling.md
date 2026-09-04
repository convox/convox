---
title: "Autoscaling"
description: "Autoscale Convox services on horizontal count, CPU, memory, and GPU with scale.autoscale triggers, and observe how the Cluster Autoscaler adds nodes per zone."
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

> A [stateful service](/configuration/volumes#per-replica-persistent-volumes) requires a fixed `scale.count` and supports none of these, including VPA. Set its replica count explicitly.

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
- Changing the GPU vendor of a deployed service requires editing `scale.gpu.vendor` in `convox.yml` and redeploying. Swapping it at runtime with `convox scale --gpu-vendor` or `convox services update --gpu-vendor` is not supported: the new vendor's resource key is added while the previous vendor's key stays in the pod spec, so scheduling stalls
- AWS Neuron (`aws.amazon.com/neuron`) is not mapped. Do not set `scale.gpu.vendor: neuron`; an unrecognized vendor falls back to `nvidia.com/gpu`, so the service requests NVIDIA GPUs instead

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

## Observing Cluster Autoscaler Scale-Ups

On AWS Racks, the Kubernetes Cluster Autoscaler adds nodes by raising the desired capacity of an EKS managed node group. Which group it grows depends on the pending pods and on the per-zone layout of the node groups.

> On Racks running [Karpenter](/configuration/scaling/karpenter), the Cluster Autoscaler manages only additional node groups, and a Rack with no additional node groups runs it at zero replicas, so the commands below return nothing and the status ConfigMap goes stale instead of being written. Additional build groups do not count toward that: a Rack whose only extra groups are additional build groups also runs the autoscaler at zero replicas, leaving those groups unmanaged. See [Cluster Autoscaler Coexistence](/configuration/scaling/karpenter#cluster-autoscaler-coexistence) for the full matrix.

### Node Group Layout

Convox creates one EKS managed node group per availability zone for primary nodes, three of them with the default [`high_availability=true`](/configuration/rack-parameters/aws/high_availability). They share one instance type and one launch template, and each group is pinned to a single subnet. The group name carries its zone, followed by an index and a 16-character random suffix with no separator between them:

```text
rackName-us-east-1a-04f7b2c9a1e6d3805
```

Treat that name as illustrative. The index comes from subnet ordering rather than from the zone list, so index `0` is not guaranteed to be the first zone alphabetically. The [dedicated build node group](/configuration/rack-parameters/aws/build_node_enabled) is not per-zone: there is one group whatever the zone count, always pinned to the first subnet. Its name carries that subnet's zone the same way (`rackName-build-us-east-1a-0...`).

[Additional node groups](/configuration/rack-parameters/aws/additional_node_groups_config) are not per-zone either. One group spans every subnet, and its name (`rackName-additional-<id>-<suffix>`) carries no zone, so you cannot read a zone out of the activity history of an additional group.

### How the Autoscaler Picks a Group

The autoscaler runs with `--expander=least-waste`, so among the groups that can host the pending pods it grows the one that leaves the least unused CPU and memory.

When Karpenter is disabled it also runs with `--balance-similar-node-groups`. That splits each scale-up across the per-zone primary groups, smallest group first, so consecutive scale-ups keep the zones within one node of each other. A group joins the split only if every pod in the batch that fits the main group also fits it. Pods that can only run in one zone, for example a pod bound to an EBS volume that already exists there or a pod carrying a zone node selector, pin their batch to that zone.

On a Karpenter Rack the discovery arguments are replaced wholesale and `--balance-similar-node-groups` is not passed, so no scale-up is ever split.

### Capacity Errors in One Zone

When EC2 cannot satisfy a launch in one zone, the Auto Scaling group retries on its own. If launches keep failing, the autoscaler puts that zone's group into backoff and sends the next scale-up to the other zones. It never routes a scale-up toward a group that is backed off.

The first backoff lasts 5 minutes. Each subsequent failure doubles it, up to a 30-minute maximum, and the ladder resets after 3 hours with no failure. The doubling only happens once a backoff window has already expired, so several scale-ups failing inside one window keep the same duration.

Each primary node group carries a single instance type, so there is no instance type fallback from the managed node groups. [Karpenter](/configuration/scaling/karpenter) selects from a set of instance types and zones on each launch.

### The Scale-Up Metric Carries No Zone

`cluster_autoscaler_scaled_up_nodes_total` is a single cluster-wide counter with no labels. It reports how many nodes the autoscaler added, never which zone, node group, or instance type they landed in. The only related counter is a GPU one, labeled by GPU resource name and GPU name, which also carries no zone, node group, or instance type.

Any `availability-zone`, `host`, or `instance-type` tag on an autoscaler series is added by whatever scrapes the metrics and belongs to the node running the single autoscaler pod, not to the nodes that were added. Grouping the scale-up counter by zone graphs where the autoscaler pod has lived. The same holds for every other autoscaler series: per-host or per-zone grouping of any of them describes the pod's host.

The autoscaler exposes its metrics on the pod through `prometheus.io/scrape` annotations rather than through a Service, so there is no scrape Service to look for.

### Finding Where Nodes Were Added

Three sources, in order of how far back they reach.

- **Auto Scaling group activity history**, about six weeks. In the EC2 console, open the Auto Scaling group behind each per-zone node group and read its Activity tab. This is the only source that survives an autoscaler restart.
- **The autoscaler log**, for as long as the current pod has been running.
- **The current node list**, for where the surviving nodes are now.

Read the log for the scale-up decisions:

```bash
$ kubectl logs -n kube-system deployment/cluster-autoscaler | grep -E 'Scale-up: setting group|Splitting scale-up between|No similar node groups found'
```

`Scale-up: setting group <group> size to <n>` prints at every verbosity level and names the group, and therefore the zone. `Splitting scale-up between <n> similar node groups: {<names>}` appears only when balancing splits a scale-up and lists the groups it went to; `No similar node groups found` appears when there is nothing to split across.

Read the node list for the current picture:

```bash
$ kubectl get nodes -L topology.kubernetes.io/zone,eks.amazonaws.com/nodegroup --sort-by=.metadata.creationTimestamp
```

### Autoscaler Status per Node Group

The autoscaler writes a status ConfigMap named `cluster-autoscaler-status` in the namespace it runs in, `kube-system` on a Convox Rack. This is the fastest way to see a zone in backoff:

```bash
$ kubectl -n kube-system get configmap cluster-autoscaler-status -o jsonpath='{.data.status}'
```

The single `status` key holds YAML with the top-level fields `time`, `autoscalerStatus`, `message`, `clusterWide`, and `nodeGroups`. Each entry under `nodeGroups` carries `name`, `health`, `scaleUp`, and `scaleDown`. For the backoff behavior above, read `scaleUp.status`, which reads `Backoff` while a group is backed off, and `scaleUp.backoffInfo`, which carries the error code and message from the failed launch. `clusterWide.health.nodeCounts` carries the registered node counts.

### Zone Spread Does Not Steer Scale-Up

The [`spreadAcrossZones`](/reference/primitives/app/service#spreadacrosszones) Service attribute keeps a Service's pods spread across zones and nodes. It shapes where pods land on nodes that already exist and does not choose which node group the autoscaler grows. Both constraints it renders are scheduler preferences rather than requirements, so a skew the scheduler cannot satisfy still places the pod; nothing is left Pending, and the autoscaler has nothing to react to.

## See Also

- [convox.yml](/configuration/convox-yml) for configuring scale defaults
- [VPA](/configuration/scaling/vpa) for automatic resource right-sizing
- [KEDA Autoscaling](/configuration/scaling/keda) for event-driven autoscaling
- [Datadog Metrics](/configuration/scaling/datadog-metrics) for Datadog-based autoscaling
- [Karpenter](/configuration/scaling/karpenter) for pod-level node provisioning as an alternative to Cluster Autoscaler (AWS only)
- [Console Autoscale Triggers](/console/autoscale-triggers)
