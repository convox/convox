---
title: "Karpenter"
slug: karpenter
url: /configuration/scaling/karpenter
---

# Karpenter

Convox supports [Karpenter](https://karpenter.sh/) as an opt-in alternative to Cluster Autoscaler for AWS EKS node provisioning. When enabled, Karpenter manages workload and build nodes through NodePools and EC2NodeClasses, delivering faster node provisioning, cost-aware instance selection, and automatic node lifecycle management.

Karpenter is **bidirectional** — `karpenter_enabled` can be toggled on and off safely, letting you try Karpenter and revert if needed without disrupting your Rack.

> Karpenter is available on **AWS only**. Karpenter parameters are rejected for GCP, Azure, and DigitalOcean Racks.

## How Karpenter Works with Convox

When Karpenter is enabled, your Rack's node provisioning is split into three tiers:

| Tier | Managed by | Node type | Purpose |
|------|-----------|-----------|---------|
| **System nodes** | EKS managed node groups (always) | ON_DEMAND | Rack control plane: API server, router, resolver, metrics-server, Karpenter controller |
| **Workload nodes** | Karpenter NodePool | Configurable | Your application Services |
| **Build nodes** | Karpenter NodePool | Configurable | `convox build` / `convox deploy` build pods |

Karpenter replaces Cluster Autoscaler for workload and build node scaling. System nodes always remain as EKS managed node groups to protect the Karpenter controller's own availability. System pods are pinned to system nodes via `nodeSelector` when Karpenter is enabled.

**Karpenter version:** `1.10.0` (pinned, not user-configurable)

### Why Karpenter Over Cluster Autoscaler

Cluster Autoscaler (CAS) works at the Auto Scaling Group (ASG) level — it can only scale groups of identical instances and reacts to pending pods by incrementing ASG desired count. Karpenter works at the pod level, directly evaluating pending pod requirements and provisioning the optimal instance type, size, and purchasing model in seconds rather than minutes.

- **Faster scaling.** Karpenter provisions nodes in response to pending pods within seconds, compared to the multi-minute feedback loop of CAS
- **Cost optimization.** Karpenter selects the cheapest instance type that satisfies pod requirements from across all allowed families and sizes
- **Node consolidation.** Underutilized nodes are automatically consolidated — Karpenter moves pods to fewer, better-utilized nodes and terminates the empty ones
- **Automatic node replacement.** Nodes are replaced after `karpenter_node_expiry` (default 30 days), keeping your fleet on current AMIs
- **Scale-to-zero builds.** The build NodePool scales to zero when no builds are running, eliminating idle build node costs
- **Multi-architecture support.** Workload node architecture is auto-detected from `node_type`, or set explicitly with `karpenter_arch`

## Enabling Karpenter

Most users enable Karpenter with a single command:

```bash
$ convox rack params set karpenter_auth_mode=true karpenter_enabled=true -r rackName
Setting parameters... OK
```

Karpenter uses a two-parameter enablement model:

1. **`karpenter_auth_mode=true`** — A **one-way migration** that prepares the EKS cluster. It migrates the cluster to `API_AND_CONFIG_MAP` access mode and applies `karpenter.sh/discovery` tags to subnets and security groups. This cannot be reversed once enabled (matching AWS EKS behavior).

2. **`karpenter_enabled=true`** — A **bidirectional toggle** that deploys the Karpenter controller, NodePools, IAM roles, and SQS interruption queue. Requires `karpenter_auth_mode=true`. Can be toggled on and off freely.

Both can be set in the same call. If setting them separately, `karpenter_auth_mode` must be set first and the update must complete before setting `karpenter_enabled`.

## Enablement Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `karpenter_auth_mode` | string | `false` | **One-way.** Migrates EKS to `API_AND_CONFIG_MAP` access mode and applies discovery tags. Cannot be set back to `false` once enabled. |
| `karpenter_enabled` | string | `false` | **Bidirectional.** Deploys Karpenter controller, NodePools, IAM roles, and SQS interruption queue. Requires `karpenter_auth_mode=true`. |

## Workload NodePool Parameters

These parameters control how Karpenter provisions nodes for your application Services.

### Instance Selection

| Parameter | Type | Default | Validation | Description |
|-----------|------|---------|------------|-------------|
| `karpenter_instance_families` | string | _(all families)_ | `^[a-z0-9]+(,[a-z0-9]+)*$` | Comma-separated EC2 instance families (e.g., `c5,m6i,r5`). All general-purpose families if unset. |
| `karpenter_instance_sizes` | string | _(all sizes)_ | `^[a-z0-9]+(,[a-z0-9]+)*$` | Comma-separated instance sizes (e.g., `large,xlarge,2xlarge`). All sizes if unset. |
| `karpenter_arch` | string | _(auto-detect)_ | `amd64`, `arm64`, `amd64,arm64`, or empty | CPU architecture. Empty = auto-detect from `node_type`. |
| `karpenter_capacity_types` | string | `on-demand` | `on-demand`, `spot`, or `on-demand,spot` | EC2 purchasing model. When both are set, Karpenter optimizes for cost and falls back to on-demand when spot is unavailable. |

### Resource Limits

| Parameter | Type | Default | Validation | Description |
|-----------|------|---------|------------|-------------|
| `karpenter_cpu_limit` | number | `100` | > 0 | Maximum total vCPUs Karpenter can provision across all workload nodes. Safety limit against runaway scaling. |
| `karpenter_memory_limit_gb` | number | `400` | > 0 | Maximum total memory (GiB) Karpenter can provision across all workload nodes. |

### Node Lifecycle and Consolidation

| Parameter | Type | Default | Validation | Description |
|-----------|------|---------|------------|-------------|
| `karpenter_consolidation_enabled` | bool | `true` | | When `true`: `WhenEmptyOrUnderutilized` (consolidates underutilized and empty nodes). When `false`: `WhenEmpty` (only removes fully empty nodes). |
| `karpenter_consolidate_after` | string | `30s` | `^\d+[smh]$` | Delay before consolidation triggers (e.g., `30s`, `5m`, `1h`). |
| `karpenter_node_expiry` | string | `720h` | `^\d+h$` or `Never` | Maximum node lifetime before automatic replacement. Default is 30 days. `Never` disables automatic replacement. |
| `karpenter_disruption_budget_nodes` | string | `10%` | `^\d+%?$` | Maximum nodes disrupted simultaneously (e.g., `10%`, `3`). |

### Storage

| Parameter | Type | Default | Validation | Description |
|-----------|------|---------|------------|-------------|
| `karpenter_node_disk` | number | `0` | >= 0 | EBS volume size in GiB for Karpenter-provisioned nodes. `0` inherits the Rack's [`node_disk`](/configuration/rack-parameters/aws/node_disk) value. |
| `karpenter_node_volume_type` | string | `gp3` | `gp2`, `gp3`, `io1`, `io2` | EBS volume type for Karpenter-provisioned nodes. |

### Labels and Taints

| Parameter | Type | Default | Validation | Description |
|-----------|------|---------|------------|-------------|
| `karpenter_node_labels` | string | _(none)_ | Comma-separated `key=value`; no double quotes; `convox.io/nodepool` reserved | Custom labels added alongside the default `convox.io/nodepool=workload` label. |
| `karpenter_node_taints` | string | _(none)_ | Comma-separated `key=value:Effect`; effect must be `NoSchedule`, `PreferNoSchedule`, or `NoExecute` | Custom taints on workload nodes. Prevents pods without matching tolerations from scheduling on these nodes. See [Using Taints to Protect Nodes](#using-taints-to-protect-nodes) below. |

## Build NodePool Parameters

These parameters control the dedicated build NodePool. The build NodePool is only created when [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled).

| Parameter | Type | Default | Validation | Description |
|-----------|------|---------|------------|-------------|
| `karpenter_build_instance_families` | string | _(workload families)_ | `^[a-z0-9]+(,[a-z0-9]+)*$` | Instance families for build nodes. Falls back to workload families if unset. |
| `karpenter_build_instance_sizes` | string | _(workload sizes)_ | `^[a-z0-9]+(,[a-z0-9]+)*$` | Instance sizes for build nodes. Falls back to workload sizes if unset. |
| `karpenter_build_capacity_types` | string | `on-demand` | `on-demand`, `spot`, or `on-demand,spot` | Purchasing model for build nodes. |
| `karpenter_build_cpu_limit` | number | `32` | > 0 | Maximum total vCPUs for the build NodePool. |
| `karpenter_build_memory_limit_gb` | number | `256` | > 0 | Maximum total memory (GiB) for the build NodePool. |
| `karpenter_build_consolidate_after` | string | `60s` | `^\d+[smh]$` | Delay before empty build nodes are consolidated. |
| `karpenter_build_node_labels` | string | _(none)_ | Comma-separated `key=value`; no double quotes; `convox-build` and `convox.io/nodepool` reserved | Extra labels added alongside default `convox-build=true` and `convox.io/nodepool=build` labels. |

### Build Node Behavior with Karpenter

When Karpenter is enabled and `build_node_enabled=true`:

- The existing EKS managed build node group is scaled to zero
- Karpenter's build NodePool provisions nodes on-demand when build pods are scheduled
- Build nodes have a `dedicated=build:NoSchedule` taint — only build pods run on them
- Build nodes scale back to zero after the last build completes (configurable via `karpenter_build_consolidate_after`, default 60s)
- Architecture is auto-detected from `build_node_type`
- The existing `build_node_min_count` parameter does not apply when Karpenter manages builds

## Advanced Configuration

### `karpenter_config` — Workload NodePool Override

For users who need access to the full Karpenter API beyond what individual parameters expose, `karpenter_config` provides a JSON escape hatch for the workload NodePool and its EC2NodeClass.

Individual `karpenter_*` parameters build the defaults. `karpenter_config` overrides them at the section level. For example, setting `nodePool.template.spec.requirements` in the config completely replaces the defaults built from `karpenter_instance_families`, `karpenter_instance_sizes`, etc.

**Input formats:** Raw JSON string, base64-encoded JSON, or a `.json` file path. Maximum 64KB.

**Structure:**

```json
{
  "nodePool": {
    "template": {
      "metadata": { "labels": { "custom-key": "custom-value" } },
      "spec": {
        "requirements": [],
        "taints": [],
        "expireAfter": "720h",
        "terminationGracePeriod": "48h"
      }
    },
    "limits": { "cpu": "200", "memory": "800Gi" },
    "disruption": {
      "consolidationPolicy": "WhenEmpty",
      "consolidateAfter": "5m",
      "budgets": [
        { "nodes": "10%" },
        { "nodes": "0", "schedule": "0 9 * * mon-fri", "duration": "8h" }
      ]
    },
    "weight": 50
  },
  "ec2NodeClass": {
    "blockDeviceMappings": [],
    "metadataOptions": { "httpTokens": "required", "httpPutResponseHopLimit": 2 },
    "tags": { "Environment": "production", "CostCenter": "engineering" },
    "amiSelectorTerms": [{ "alias": "al2023@latest" }],
    "userData": "...",
    "detailedMonitoring": true,
    "associatePublicIPAddress": false,
    "instanceStorePolicy": "RAID0"
  }
}
```

**Fields only available via `karpenter_config`:**

| Field | Description |
|-------|-------------|
| `nodePool.template.spec.terminationGracePeriod` | Time allowed for graceful pod eviction before force termination |
| `nodePool.disruption.budgets[].schedule` | Cron-based disruption windows (e.g., no disruptions during business hours) |
| `ec2NodeClass.userData` | Custom EC2 user data script |
| `ec2NodeClass.detailedMonitoring` | Enable CloudWatch detailed monitoring |
| `ec2NodeClass.associatePublicIPAddress` | Associate public IP with Karpenter nodes |
| `ec2NodeClass.instanceStorePolicy` | Instance store disk policy (e.g., `RAID0`) |
| `ec2NodeClass.amiSelectorTerms` | Custom AMI selection (default: `al2023@latest`) |
| `ec2NodeClass.metadataOptions` | EC2 instance metadata options (IMDSv2 settings) |
| `ec2NodeClass.blockDeviceMappings` | Custom EBS volume configuration beyond `karpenter_node_disk` / `karpenter_node_volume_type` |

**Protected fields** (managed by Convox, cannot be overridden):

| Field | Reason |
|-------|--------|
| `ec2NodeClass.role` | IAM role managed by Convox |
| `ec2NodeClass.instanceProfile` | IAM instance profile managed by Convox |
| `ec2NodeClass.subnetSelectorTerms` | Subnet discovery tags managed by Convox |
| `ec2NodeClass.securityGroupSelectorTerms` | Security group discovery tags managed by Convox |
| `nodePool.template.spec.nodeClassRef` | Must reference Convox-managed EC2NodeClass |
| `nodePool.template.metadata.labels["convox.io/nodepool"]` | Reserved Convox label |
| `ec2NodeClass.tags.Name` | Reserved tag (forced to `{rack}/karpenter/workload`) |
| `ec2NodeClass.tags.Rack` | Reserved tag (forced to Rack name) |

**Example: maintenance window with no disruptions during business hours**

```bash
$ convox rack params set karpenter_config='{"nodePool":{"disruption":{"budgets":[{"nodes":"10%"},{"nodes":"0","schedule":"0 9 * * mon-fri","duration":"8h"}]}}}' -r rackName
Setting parameters... OK
```

### `additional_karpenter_nodepools_config` — Custom NodePools

Creates additional NodePools beyond the built-in workload and build pools. Each entry in the JSON array produces its own NodePool + EC2NodeClass pair with the same infrastructure settings (subnet discovery, security groups, IAM role) as the workload pool.

Use this for dedicated GPU pools, tenant isolation, specialized instance requirements, or batch processing pools.

**Input formats:** Raw JSON string, base64-encoded JSON, or a `.json` file path.

**Per-pool fields:**

| Field | Type | Default | Required | Description |
|-------|------|---------|----------|-------------|
| `name` | string | | **yes** | Unique pool identifier. Lowercase alphanumeric with dashes, max 63 chars. Reserved names: `workload`, `build`, `default`, `system`. |
| `instance_families` | string | _(all)_ | no | Comma-separated EC2 families (e.g., `g5,g6`). |
| `instance_sizes` | string | _(all)_ | no | Comma-separated sizes (e.g., `xlarge,2xlarge`). |
| `capacity_types` | string | `on-demand` | no | `on-demand`, `spot`, or `on-demand,spot`. |
| `arch` | string | `amd64` | no | `amd64`, `arm64`, or `amd64,arm64`. |
| `cpu_limit` | integer | `100` | no | Maximum total vCPUs for this pool. |
| `memory_limit_gb` | integer | `400` | no | Maximum total memory (GiB) for this pool. |
| `consolidation_policy` | string | `WhenEmptyOrUnderutilized` | no | `WhenEmpty` or `WhenEmptyOrUnderutilized`. |
| `consolidate_after` | string | `30s` | no | Delay before consolidation (e.g., `30s`, `5m`). |
| `node_expiry` | string | `720h` | no | Max node lifetime. `Never` to disable. |
| `disruption_budget_nodes` | string | `10%` | no | Max nodes disrupted simultaneously. |
| `disk` | integer | _(workload value)_ | no | EBS volume size in GiB. `0` inherits workload pool disk. |
| `volume_type` | string | `gp3` | no | `gp2`, `gp3`, `io1`, `io2`. |
| `weight` | integer | _(unset)_ | no | Scheduling weight (0-100). Higher = preferred. |
| `labels` | string | _(none)_ | no | Comma-separated `key=value`. `convox.io/nodepool` is reserved. |
| `taints` | string | _(none)_ | no | Comma-separated `key=value:Effect`. Valid effects: `NoSchedule`, `PreferNoSchedule`, `NoExecute`. Prevents pods without matching tolerations from scheduling on these nodes. See [Using Taints to Protect Nodes](#using-taints-to-protect-nodes) below. |
| `dedicated` | bool | `false` | no | When `true`, adds a `dedicated-node={name}:NoSchedule` taint automatically. Services with `nodeSelectorLabels: { convox.io/nodepool: {name} }` get the matching toleration injected. Simpler alternative to manual `taints` for pool isolation. |

Every custom pool automatically gets a `convox.io/nodepool={name}` label. Target Services to a custom pool using `nodeSelectorLabels` in `convox.yml`:

```yaml
services:
  ml-worker:
    build: .
    nodeSelectorLabels:
      convox.io/nodepool: gpu
```

**Example: Dedicated GPU pool (simple path)**

```bash
$ convox rack params set additional_karpenter_nodepools_config='[{"name":"gpu","instance_families":"g5,g6","dedicated":true}]' -r rackName
Setting parameters... OK
```

```yaml
# convox.yml
services:
  gpu-worker:
    build: .
    scale:
      gpu:
        count: 1
        vendor: nvidia
    nodeSelectorLabels:
      convox.io/nodepool: gpu
```

With `dedicated: true`, only services targeting this pool (via `nodeSelectorLabels`) can schedule on it. No manual taint configuration needed. `convox run gpu-worker` also inherits the placement automatically.

**Example: GPU pool and high-memory pool (advanced)**

```bash
$ convox rack params set additional_karpenter_nodepools_config='[{"name":"gpu","instance_families":"g5,g6","capacity_types":"on-demand","cpu_limit":64,"memory_limit_gb":256,"taints":"nvidia.com/gpu=true:NoSchedule","disk":200},{"name":"high-mem","instance_families":"r5,r6i","instance_sizes":"xlarge,2xlarge,4xlarge","capacity_types":"on-demand,spot","cpu_limit":200,"memory_limit_gb":1600,"labels":"pool=high-mem"}]' -r rackName
Setting parameters... OK
```

### Using Taints to Protect Nodes

Without taints, any pod can land on a custom NodePool's nodes if they have spare capacity — even pods that don't need those resources. For example, a basic web service could get scheduled to an expensive GPU instance. Taints prevent this by rejecting pods that lack a matching toleration.

> **Important:** `convox.yml` does not have a `tolerations` field. You cannot manually specify tolerations in your Service definition. Instead, tolerations are handled automatically through the mechanisms described below.

#### GPU workloads

For GPU pools with an `nvidia.com/gpu` taint, use [`scale.gpu`](/configuration/scaling/autoscaling) in `convox.yml` to request GPU resources:

```yaml
services:
  ml-worker:
    build: .
    scale:
      gpu:
        count: 1
        vendor: nvidia
    nodeSelectorLabels:
      convox.io/nodepool: gpu
```

When a Service declares `scale.gpu`, Convox adds `nvidia.com/gpu` to the pod's resource requests. Kubernetes' `ExtendedResourceToleration` admission controller then auto-adds the matching toleration. No manual toleration configuration is needed.

You must also enable the NVIDIA device plugin on the Rack:

```bash
$ convox rack params set nvidia_device_plugin_enable=true -r rackName
Setting parameters... OK
```

#### Dedicated pools

The simplest way to isolate a pool is `dedicated: true`. This auto-applies a `dedicated-node={name}:NoSchedule` taint and Convox auto-injects the matching toleration for any Service with `nodeSelectorLabels: { convox.io/nodepool: {name} }`. No external webhooks or manual taint configuration needed.

#### Non-GPU custom taints

For custom taints beyond `dedicated` (e.g., tenant isolation with specific taint keys), Convox does not auto-inject tolerations. Pods targeting pools with custom non-GPU taints will need tolerations added through an external mechanism such as a Kubernetes mutating admission webhook. For most non-GPU use cases, using `dedicated: true` or `labels` + `nodeSelectorLabels` without taints is the recommended approach — Karpenter only provisions nodes for pods that need them, so unwanted pods won't cause unnecessary scaling.

#### DaemonSets on tainted nodes

Node-level DaemonSets (fluentd, `aws-node`, `kube-proxy`, `ebs-csi-node`, `efs-csi-node`, `eks-pod-identity-agent`) use `operator: Exists` tolerations and are **not affected** by custom taints. They will continue to run on all nodes, including tainted custom NodePool nodes.

You can also use a JSON file:

```bash
$ convox rack params set additional_karpenter_nodepools_config=/path/to/nodepools.json -r rackName
Setting parameters... OK
```

## System Node Behavior

System nodes are **always** EKS managed node groups, regardless of whether Karpenter is enabled. This ensures the Karpenter controller itself and other critical Rack components cannot be disrupted by Karpenter's own consolidation or scaling decisions.

When `karpenter_enabled=true`:

- System node capacity type is forced to `ON_DEMAND`
- System nodes get the `convox.io/system-node=true` label
- The following pods are pinned to system nodes via `nodeSelector`:
  - Rack API server
  - Router (both public and internal)
  - Resolver
  - Metrics server
  - Cluster Autoscaler (if running)
  - Karpenter controller
- Fluentd DaemonSet is **not** pinned — it runs on all nodes for log collection

> The `convox.io/system-node=true` label is tied to `karpenter_auth_mode` (not `karpenter_enabled`) to ensure labels persist during enable/disable transitions.

## Cluster Autoscaler Coexistence

Karpenter and Cluster Autoscaler (CAS) can coexist when additional (non-Karpenter) node groups are present:

| Scenario | CAS state | CAS targeting |
|----------|----------|---------------|
| Karpenter enabled, no additional node groups | Scaled to 0 replicas | N/A |
| Karpenter enabled, additional node groups (all `dedicated=true`) | Running (1 replica, pinned to system nodes) | Explicit `--nodes` per additional ASG (no auto-discovery) |
| Karpenter disabled | Running (normal) | Auto-discovery |

Enabling Karpenter requires all existing [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config) entries to have `dedicated=true`. This prevents scheduling overlap between CAS-managed and Karpenter-managed nodes.

## Disabling Karpenter

Setting `karpenter_enabled=false` cleanly reverses the deployment:

1. Karpenter controller drains workload and build nodes (5-minute graceful drain window)
2. All Karpenter NodePools, EC2NodeClasses, IAM resources, and SQS queue are destroyed
3. The managed build node group scales back up from zero to `build_node_min_count`
4. Cluster Autoscaler resumes normal auto-discovery mode
5. System pod `nodeSelector` for `convox.io/system-node` is removed

> `karpenter_auth_mode` cannot be reverted. The EKS access config migration and discovery tags remain. This is safe and has no cost or operational impact — it means the cluster is ready for Karpenter to be re-enabled at any time.

## Constraints and Limitations

- **AWS only.** Karpenter parameters are rejected for GCP, Azure, DigitalOcean, and other providers.
- **Rack name length.** Racks with Karpenter enabled are limited to 26-character names (due to derived AWS resource names like `{name}-karpenter-nodes`).
- **BYOVPC shared VPCs.** If multiple Racks share a VPC (BYOVPC into another Rack's VPC), only one may enable Karpenter due to `karpenter.sh/discovery` tag collision on shared subnets and security groups.
- **`karpenter_auth_mode` is one-way.** Cannot be reverted once enabled (matching AWS EKS behavior for access config migration).
- **Additional node groups must be `dedicated=true`.** Required when enabling Karpenter to prevent scheduling overlap between CAS and Karpenter.

## See Also

- [Workload Placement](/configuration/scaling/workload-placement) for node group-based placement with Cluster Autoscaler
- [Autoscaling](/configuration/scaling/autoscaling) for horizontal pod autoscaling and GPU scaling
- [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) for enabling the dedicated build node
- [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config) for custom EKS managed node groups
- [node_type](/configuration/rack-parameters/aws/node_type) for primary node instance type
- [node_disk](/configuration/rack-parameters/aws/node_disk) for primary node disk size
- [nvidia_device_plugin_enable](/configuration/rack-parameters/aws/nvidia_device_plugin_enable) for GPU workloads
