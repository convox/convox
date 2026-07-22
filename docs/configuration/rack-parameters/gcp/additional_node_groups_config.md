---
title: "additional_node_groups_config"
description: "The additional_node_groups_config GCP rack parameter defines additional GKE node pools with specific machine types, capacity types, scaling, GPU accelerators, and custom labels."
slug: additional_node_groups_config
url: /configuration/rack-parameters/gcp/additional_node_groups_config
---

# additional_node_groups_config

## Description
The `additional_node_groups_config` parameter allows you to configure additional customized node pools for your GKE cluster. This feature enables more granular control over your Kubernetes infrastructure by letting you define node pools with specific machine types, capacity types, scaling parameters, GPU accelerators, and custom labels.

When combined with node selector configurations, you can optimize workload placement, improve cost efficiency, run GPU workloads, and separate operational concerns within your cluster.

## Default Value
The default value for `additional_node_groups_config` is an empty array.

## Use Cases
- **Workload-Specific Optimization**: Create node pools tailored to specific workload requirements (e.g., CPU-intensive, memory-intensive, GPU, or batch processing workloads).
- **GPU Workloads**: Run machine learning, inference, or rendering workloads on nodes with NVIDIA GPUs.
- **Cost Optimization**: Configure certain node pools to use Spot VMs for non-critical workloads while maintaining on-demand VMs for mission-critical services.
- **Isolation**: Segregate workloads by dedicating specific node pools to particular services.

## Configuration Format
The `additional_node_groups_config` parameter takes a JSON array of node pool configurations. Each node pool configuration is a JSON object with the following fields:

| Field | Required | Description | Default |
|-------|----------|-------------|---------|
| `type` | Yes | The GCP machine type to use for the node pool (e.g., `n1-standard-4`, `g2-standard-8`) |  |
| `disk` | No | The boot disk size in GB for the nodes | Same as main node disk (default: 100) |
| `disk_type` | No | The boot disk type (e.g., `pd-balanced`, `pd-ssd`, `pd-standard`) | `pd-balanced` |
| `capacity_type` | No | Whether to use on-demand or spot VMs. Accepts `ON_DEMAND` or `SPOT` (`Regular` and `Spot` are also accepted, for configs shared with Azure) | `ON_DEMAND` |
| `min_size` | No | Minimum number of nodes per zone. `0` is allowed for scale-to-zero pools | 1 |
| `max_size` | No | Maximum number of nodes per zone | 100 |

> **Counts are per zone, not totals.** GCP racks use regional GKE clusters, so `min_size` and `max_size` apply to each zone in the region. In a 3-zone region, `min_size: 1` runs 3 nodes and `max_size: 100` allows up to 300. This differs from AWS and Azure, where the same values are totals. For expensive pools (such as GPU pools), set `min_size: 0` or pin a single zone with `zones`.
| `label` | No | Custom label value for the node pool. Applied as `convox.io/label: <label-value>` | None |
| `id` | No | A unique integer identifier for the node pool that persists across updates | Auto-generated |
| `tags` | No | Custom GCP resource labels specified as comma-separated key-value pairs (e.g., `environment=production,team=backend`) | None |
| `dedicated` | No | When `true`, only services with matching node pool labels will be scheduled on these nodes (adds a `dedicated-node` NoSchedule taint) | `false` |
| `zones` | No | Comma-separated list of GCP zones (e.g., `us-east1-b,us-east1-c`) | None (region default) |
| `gpu_type` | No | NVIDIA accelerator type to attach (e.g., `nvidia-l4`, `nvidia-tesla-t4`). When set, GKE installs the NVIDIA driver and device plugin automatically and taints the pool with `nvidia.com/gpu:NoSchedule` | None |
| `gpu_count` | No | Number of GPUs to attach per node (only applies when `gpu_type` is set) | 1 |

## Setting Parameters
To set the `additional_node_groups_config` parameter, there are several methods:

### Using a JSON File (Recommended)
```bash
$ convox rack params set additional_node_groups_config=/path/to/node-config.json -r rackName
Updating parameters... OK
```

The JSON file should be structured as follows:
```json
[
  {
    "id": 101,
    "type": "n1-standard-4",
    "disk": 100,
    "capacity_type": "ON_DEMAND",
    "min_size": 1,
    "max_size": 3,
    "label": "app-workers",
    "tags": "environment=production,team=backend"
  }
]
```

### Using a Raw JSON String
```bash
$ convox rack params set 'additional_node_groups_config=[{"id":101,"type":"n1-standard-4","min_size":1,"max_size":3,"label":"app-workers"}]' -r rackName
Updating parameters... OK
```

## GPU Node Pools

To run GPU workloads, set `gpu_type` (and optionally `gpu_count`) on a node pool. GKE manages the NVIDIA driver and device plugin for you, and automatically applies the `nvidia.com/gpu:NoSchedule` taint. Convox services that request GPUs (via `scale.gpu.count` in `convox.yml`) get the matching resource request and toleration automatically, so they schedule onto these nodes.

Node pools use the rack's node service account. For workloads that call GCP APIs (including GPU workloads running third-party code), use [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity) rather than relying on node credentials.

The following example creates a scale-to-zero Spot pool of `g2-standard-8` machines, each with one NVIDIA L4 GPU:

```json
[
  {
    "id": 200,
    "type": "g2-standard-8",
    "capacity_type": "SPOT",
    "min_size": 0,
    "max_size": 3,
    "label": "gpu-workers",
    "gpu_type": "nvidia-l4",
    "gpu_count": 1
  }
]
```

A matching `convox.yml` service:
```yaml
services:
  inference:
    scale:
      gpu:
        count: 1
```

> **Zone availability**: A given accelerator type is only available in specific zones. If the pool's zones (the rack region's default zones, or the `zones` you specify) do not offer the requested `gpu_type`, GKE will fail to create the pool. Consult the [GCP GPU regions and zones documentation](https://cloud.google.com/compute/docs/gpus/gpu-regions-zones) and set `zones` accordingly.

## Node Pool Identification

### Using the `id` Field

The `id` field ensures that node pools preserve their identity during configuration updates:

- Each node pool should have a unique integer identifier
- Using the `id` field prevents unnecessary recreation of node pools when making changes to their configuration
- Consistent `id` values help maintain stable infrastructure during updates

> **Changing `type`, `disk`, or `disk_type` on an existing pool recreates it** (destroy then create), which causes a temporary capacity gap for workloads on that pool. To avoid downtime, add a new pool with a new `id`, migrate workloads, then remove the old entry.

## Spot VM Considerations

When using `capacity_type: "SPOT"`:

- GCP Spot VMs can be preempted at any time when GCP needs the capacity back
- Spot VMs are best suited for fault-tolerant, stateless workloads

## Using Node Pools with Services
To target specific services to run on particular node pools, use the `nodeSelectorLabels` field in your `convox.yml` file:

```yaml
services:
  web:
    nodeSelectorLabels:
      convox.io/label: app-workers
```

This will ensure that the `web` service is scheduled only on nodes with the label `convox.io/label: app-workers`.

## Additional Information
When using dedicated node pools (with `dedicated: true`), only services with matching node selector labels will be scheduled on those nodes. This provides strong isolation for workloads with specific requirements.
