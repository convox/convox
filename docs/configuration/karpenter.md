# Karpenter Integration

Karpenter is an open-source Kubernetes node autoscaler that provisions right-sized compute resources in response to pending pods. Convox supports Karpenter as an opt-in alternative to the default Cluster Autoscaler (CAS) for AWS racks.

## Overview

When `karpenter_enabled=true`:

- **Karpenter controller** runs on dedicated system nodes (EKS managed node groups)
- **Workload pods** are scheduled on Karpenter-provisioned nodes via NodePools
- **System services** (router, API, Karpenter itself) remain on managed system nodes
- **Cluster Autoscaler** is scaled to 0 replicas (or manages only `additional_node_groups` if present)
- **Build nodes** (when `build_node_enabled=true`) use a dedicated Karpenter NodePool

The feature is **bidirectional** — you can toggle between CAS and Karpenter at runtime.

## Enabling Karpenter

```bash
convox rack params set karpenter_enabled=true
```

This single command:
1. Deploys Karpenter controller and CRDs on system nodes
2. Creates workload NodePool and EC2NodeClass
3. Scales CAS to 0 replicas (or switches to explicit ASG mode if `additional_node_groups` exist)
4. Scales workload node groups to min=0/max=0 (system nodes remain active)
5. Forces all system nodes to ON_DEMAND capacity type

### What to expect during the transition

**Pod disruption:** Existing pods on workload node groups (HA index 2, build nodes) are terminated by the ASG scale-down. Karpenter is already active at this point and will provision replacement nodes for any pending pods, but expect 60-90 seconds of rescheduling delay.

**Brief autoscaler overlap:** For approximately 30 seconds during the apply, both CAS (still in auto-discovery mode) and Karpenter (newly deployed) may respond to pending pods. If a pending pod triggers both, the result is temporary over-provisioning that self-corrects as each autoscaler consolidates. This window is short and benign.

**Karpenter-provisioned instances:** Once Karpenter is active, it provisions EC2 instances tagged with `kubernetes.io/cluster/<rack-name>: owned`. These instances are managed exclusively by the Karpenter controller.

## Disabling Karpenter (Rollback to CAS)

```bash
convox rack params set karpenter_enabled=false
```

This reverses the process:
1. Restores CAS with auto-discovery mode
2. Restores managed node group scaling (min/max return to normal)
3. Removes Karpenter NodePools (Karpenter drains nodes respecting PDBs)
4. Removes Karpenter controller, CRDs, and IAM resources

### Rollback caveats

**Karpenter-managed instances:** When the Karpenter controller is removed, any EC2 instances it provisioned become unmanaged. Karpenter's finalizers attempt to drain and terminate these nodes during NodePool deletion, but if the controller is removed before all nodes are cleaned up, orphaned instances may remain. These are tagged with `kubernetes.io/cluster/<rack-name>: owned` and can be identified and terminated manually via the AWS console or CLI.

**Partial apply failure:** If a rollback apply fails mid-way, re-run the update. Terraform's idempotency handles partial state. If Karpenter resources (IAM roles, SQS queues) persist after a failed rollback, the next successful apply will clean them up.

## Parameters

### Core

| Parameter | Default | Description |
|-----------|---------|-------------|
| `karpenter_enabled` | `false` | Enable Karpenter node autoscaling |
| `karpenter_config` | _(none)_ | JSON overrides for NodePool/EC2NodeClass (see [Advanced Configuration](#advanced-configuration)) |

### Instance Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `karpenter_instance_families` | _(all)_ | Comma-separated EC2 instance families (e.g., `c5,c6i,m5`) |
| `karpenter_instance_sizes` | _(all)_ | Comma-separated instance sizes (e.g., `large,xlarge`) |
| `karpenter_capacity_types` | `on-demand` | `on-demand`, `spot`, or `on-demand,spot` |
| `karpenter_arch` | `amd64` | `amd64`, `arm64`, or `amd64,arm64` |

### Resource Limits

| Parameter | Default | Description |
|-----------|---------|-------------|
| `karpenter_cpu_limit` | `100` | Max total vCPUs Karpenter can provision |
| `karpenter_memory_limit_gb` | `400` | Max total memory (GiB) Karpenter can provision |

### Consolidation and Disruption

| Parameter | Default | Description |
|-----------|---------|-------------|
| `karpenter_consolidation_enabled` | `true` | Enable underutilized/empty node consolidation |
| `karpenter_consolidate_after` | `30s` | Time before consolidation triggers |
| `karpenter_node_expiry` | `720h` | Max node lifetime (30 days). `Never` to disable. |
| `karpenter_disruption_budget_nodes` | `10%` | Max nodes disrupted simultaneously |

### Node Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `karpenter_node_disk` | `0` | EBS volume size (GiB). `0` = use `node_disk` value. |
| `karpenter_node_volume_type` | `gp3` | EBS volume type |
| `karpenter_node_labels` | _(none)_ | Custom labels on workload nodes (comma-separated `key=value`) |
| `karpenter_node_taints` | _(none)_ | Custom taints on workload nodes (comma-separated `key=value:Effect`) |

### Build NodePool (requires `build_node_enabled=true`)

Build nodes always use **amd64** architecture. This is not configurable because most container build toolchains and base images target amd64. If you set `karpenter_arch=arm64` for workloads, builds will still run on amd64 nodes.

| Parameter | Default | Description |
|-----------|---------|-------------|
| `karpenter_build_instance_families` | _(all)_ | Instance families for build pool |
| `karpenter_build_instance_sizes` | _(all)_ | Instance sizes for build pool |
| `karpenter_build_capacity_types` | `on-demand` | Capacity types for build pool |
| `karpenter_build_cpu_limit` | `32` | Max vCPUs for build pool |
| `karpenter_build_consolidate_after` | `60s` | Time before empty build nodes are removed |
| `karpenter_build_node_labels` | _(none)_ | Extra labels on build nodes (on top of `convox-build=true`) |

## Advanced Configuration

The individual `karpenter_*` parameters cover the most common settings. For full control over the Karpenter NodePool and EC2NodeClass specs, use `karpenter_config` to override any section.

### How it works

1. Convox builds a default NodePool + EC2NodeClass from the individual `karpenter_*` parameters
2. Your `karpenter_config` JSON overrides specific sections (section-level merge)
3. Convox forces protected infrastructure fields after merge (you can't break your rack)

### Usage

Create a JSON file (e.g., `karpenter-config.json`):

```json
{
  "nodePool": {
    "template": {
      "metadata": {
        "labels": {
          "team": "platform"
        }
      },
      "spec": {
        "requirements": [
          {"key": "karpenter.sh/capacity-type", "operator": "In", "values": ["on-demand", "spot"]},
          {"key": "kubernetes.io/arch", "operator": "In", "values": ["amd64"]},
          {"key": "karpenter.k8s.aws/instance-family", "operator": "In", "values": ["c5", "c6i", "m5", "m6i"]},
          {"key": "karpenter.k8s.aws/instance-generation", "operator": "Gt", "values": ["4"]}
        ],
        "expireAfter": "168h",
        "terminationGracePeriod": "48h"
      }
    },
    "disruption": {
      "consolidationPolicy": "WhenEmptyOrUnderutilized",
      "consolidateAfter": "60s",
      "budgets": [
        {"nodes": "10%"},
        {"nodes": "0", "schedule": "0 9 * * 1-5", "duration": "8h"}
      ]
    },
    "limits": {
      "cpu": "200",
      "memory": "800Gi"
    }
  },
  "ec2NodeClass": {
    "blockDeviceMappings": [{
      "deviceName": "/dev/xvda",
      "ebs": {
        "volumeType": "gp3",
        "volumeSize": "100Gi",
        "iops": 5000,
        "throughput": 250,
        "encrypted": true
      }
    }],
    "detailedMonitoring": true,
    "tags": {
      "CostCenter": "engineering"
    }
  }
}
```

Apply:

```bash
convox rack params set karpenter_config=karpenter-config.json
```

### Override behavior

| Section | Behavior |
|---------|----------|
| `nodePool.template.metadata.labels` | **Merged** — your labels added alongside `convox.io/nodepool: workload` |
| `nodePool.template.spec.requirements` | **Replaced** — if you provide requirements, they fully replace the defaults. Include capacity-type and arch if needed. |
| `nodePool.template.spec.taints` | **Replaced** |
| `nodePool.template.spec.expireAfter` | **Replaced** |
| `nodePool.template.spec.terminationGracePeriod` | **Added** (no default) |
| `nodePool.limits` | **Replaced** |
| `nodePool.disruption` | **Replaced entirely** — see note below |
| `nodePool.weight` | **Added** (no default) |
| `ec2NodeClass.blockDeviceMappings` | **Replaced** |
| `ec2NodeClass.metadataOptions` | **Replaced** |
| `ec2NodeClass.amiSelectorTerms` | **Replaced** — allows custom AMIs |
| `ec2NodeClass.tags` | **Merged** — your tags added alongside Name, Rack, and rack-level tags |
| `ec2NodeClass.userData` | **Added** (no default) |
| `ec2NodeClass.detailedMonitoring` | **Added** (no default) |
| `ec2NodeClass.associatePublicIPAddress` | **Added** (no default) |
| `ec2NodeClass.instanceStorePolicy` | **Added** (no default) |

**Disruption override note:** When you provide `nodePool.disruption` in `karpenter_config`, it replaces the **entire** disruption object — including `consolidationPolicy`, `consolidateAfter`, and `budgets`. The individual parameters (`karpenter_consolidation_enabled`, `karpenter_consolidate_after`, `karpenter_disruption_budget_nodes`) are ignored. You must include all three fields in your override. If you only set `consolidationPolicy`, Karpenter's own server-side defaults apply for the missing fields (not Convox's configured defaults).

### Protected fields (cannot be overridden)

These are managed by Convox and blocked at validation:

- `ec2NodeClass.role` — Karpenter node IAM role
- `ec2NodeClass.instanceProfile` — managed by Karpenter
- `ec2NodeClass.subnetSelectorTerms` — uses discovery tags
- `ec2NodeClass.securityGroupSelectorTerms` — uses discovery tags
- `ec2NodeClass.tags.Name` / `ec2NodeClass.tags.Rack` — reserved tag keys (case-insensitive)
- `nodePool.template.spec.nodeClassRef` — links NodePool to its EC2NodeClass

### Interaction with individual parameters

When `karpenter_config` is empty (default), individual `karpenter_*` parameters fully control the configuration. When `karpenter_config` is provided, it overrides at the section level:

- `karpenter_instance_families=c5,c6i` sets default requirements
- `karpenter_config` with `nodePool.template.spec.requirements` **replaces** those defaults entirely
- `karpenter_node_labels=team=backend` sets default labels
- `karpenter_config` with `nodePool.template.metadata.labels` **merges** with those labels

## Custom NodePools

For workloads that need dedicated node configurations (GPU, high-memory, team isolation, etc.), use `additional_karpenter_nodepools_config` to create custom Karpenter NodePools. This follows the same pattern as `additional_node_groups_config`.

### Usage

Create a JSON file (e.g., `karpenter-pools.json`):

```json
[
  {
    "name": "gpu",
    "instance_families": "g5,g6",
    "instance_sizes": "xlarge,2xlarge",
    "capacity_types": "on-demand",
    "cpu_limit": 64,
    "memory_limit_gb": 256,
    "labels": "gpu=true,workload-type=ml",
    "taints": "gpu=true:NoSchedule",
    "disk": 100
  },
  {
    "name": "highmem",
    "instance_families": "r5,r6i",
    "capacity_types": "on-demand,spot",
    "cpu_limit": 200,
    "memory_limit_gb": 1600,
    "labels": "pool=highmem",
    "weight": 10
  }
]
```

Apply it:

```bash
convox rack params set additional_karpenter_nodepools_config=karpenter-pools.json
```

You can also pass raw JSON or base64-encoded JSON inline.

### Schema

Each NodePool object supports:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | _(required)_ | Unique name (lowercase, alphanumeric, dashes). Cannot be `workload`, `build`, `default`, or `system`. |
| `instance_families` | string | `""` (all) | Comma-separated EC2 families (e.g., `g5,g6`) |
| `instance_sizes` | string | `""` (all) | Comma-separated sizes (e.g., `xlarge,2xlarge`) |
| `capacity_types` | string | `on-demand` | `on-demand`, `spot`, or `on-demand,spot` |
| `arch` | string | `amd64` | `amd64`, `arm64`, or `amd64,arm64` |
| `cpu_limit` | int | `100` | Max vCPUs this pool can provision |
| `memory_limit_gb` | int | `400` | Max memory (GiB) this pool can provision |
| `consolidation_policy` | string | `WhenEmptyOrUnderutilized` | `WhenEmpty` or `WhenEmptyOrUnderutilized` |
| `consolidate_after` | string | `30s` | Duration before consolidation |
| `node_expiry` | string | `720h` | Max node lifetime, or `Never` |
| `disruption_budget_nodes` | string | `10%` | Max disrupted nodes |
| `disk` | int | `0` (use default) | EBS volume size (GiB) |
| `volume_type` | string | `gp3` | `gp2`, `gp3`, `io1`, `io2` |
| `labels` | string | `""` | Comma-separated `key=value` pairs for node labels |
| `taints` | string | `""` | Comma-separated `key=value:Effect` (Effect: `NoSchedule`, `PreferNoSchedule`, `NoExecute`) |
| `weight` | int | _(not set)_ | NodePool priority (higher = preferred, 0-100). When not set, Karpenter uses default priority. `0` = lowest explicit priority. |

### Targeting Custom NodePools

To schedule pods on a custom NodePool, use node selectors or affinities matching your custom labels. For tainted pools, add matching tolerations to your service.

Example `convox.yml` for a GPU workload targeting the `gpu` pool above:

```yaml
services:
  ml-training:
    build: .
    nodeSelector:
      gpu: "true"
    tolerations:
      - key: gpu
        operator: Equal
        value: "true"
        effect: NoSchedule
```

### Notes

- Each custom NodePool gets its own EC2NodeClass (separate disk/volume config)
- All custom NodePools share the same Karpenter node IAM role and subnet/SG discovery tags
- Custom NodePools are only created when `karpenter_enabled=true`
- The `convox.io/nodepool: <name>` label is automatically added to every custom NodePool

## Existing Parameter Behavior Changes

When `karpenter_enabled=true`, some existing parameters change behavior:

| Parameter | With Karpenter |
|-----------|---------------|
| `node_type` | Applies only to system node groups. Workload nodes use `karpenter_instance_families`/`karpenter_instance_sizes`. |
| `node_disk` | Applies to system nodes. Karpenter nodes use `karpenter_node_disk` (defaults to `node_disk`). |
| `node_capacity_type` | **Overridden to ON_DEMAND for system nodes.** This prevents spot reclamation of Karpenter controller nodes. Workload nodes use `karpenter_capacity_types`. |
| `build_node_type` | Ignored; use `karpenter_build_instance_families`. |
| `schedule_rack_scale_down/up` | **No effect.** ASG schedules don't apply to Karpenter nodes. |
| `min_on_demand_count` / `max_on_demand_count` | **No effect.** MIXED mode not applicable with Karpenter. |

## Architecture

### System Nodes

- **HA mode**: 2 system node groups (indices 0 and 1) with `convox.io/system-node=true` label. Index 2 scaled to 0 (kept for rollback).
- **Non-HA mode**: 1 system node group.
- Karpenter controller runs with 2 replicas (HA) or 1 replica (non-HA), pinned to system nodes.

### SPOT/MIXED Capacity Type Override

When enabling Karpenter on a rack using SPOT or MIXED `node_capacity_type`, all system nodes are forced to ON_DEMAND. This triggers a node group replacement (the `random_id` keeper changes). **Plan for brief disruption during this transition.**

### Additional Node Groups + Karpenter

If `additional_node_groups_config` is set alongside `karpenter_enabled=true`:
- CAS continues running with 1 replica
- CAS switches from auto-discovery to explicit ASG targeting (manages only additional groups)
- Karpenter manages workload scheduling
- Both autoscalers coexist safely

## Troubleshooting

### Partial Apply Failure

If `convox rack update` fails partway through enabling/disabling Karpenter, re-run the update. Terraform's idempotency handles partial state and will complete the transition.

### cert-manager Considerations

cert-manager pods may run on Karpenter-managed nodes and are subject to consolidation. cert-manager pods are stateless and reschedule quickly. This is a pre-existing concern with CAS node rotations — Karpenter just makes node turnover more frequent.

### KEDA Interaction

When KEDA scales pods rapidly, Karpenter provisions nodes to accommodate them. If KEDA scales down to 0 and then back up, expect ~60-90 seconds of cold-start delay for node provisioning (same as CAS via ASG scaling).

## Known Limitations

- `schedule_rack_scale_down/up` has no effect with Karpenter (ASG schedules don't apply to Karpenter-managed nodes)
- Karpenter `karpenter_enabled` is not an immutable parameter — it can be toggled at any time
- DaemonSets without resource requests (e.g., Fluentd) may cause Karpenter to slightly undersize nodes
- Karpenter-provisioned nodes don't use custom `user_data` or `user_data_url` settings
