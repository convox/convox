---
title: "Karpenter"
description: "Karpenter is an opt-in alternative to Cluster Autoscaler for AWS EKS node provisioning, with faster provisioning and cost-aware instance selection."
slug: karpenter
url: /configuration/scaling/karpenter
---

# Karpenter

Convox supports [Karpenter](https://karpenter.sh/) as an opt-in alternative to Cluster Autoscaler for AWS EKS node provisioning. When enabled, Karpenter manages workload and build nodes through NodePools and EC2NodeClasses, delivering faster node provisioning, cost-aware instance selection, and automatic node lifecycle management.

Karpenter is **bidirectional**. `karpenter_enabled` can be toggled on and off safely, letting you try Karpenter and revert if needed without disrupting your Rack.

> Karpenter is available on **AWS only**. Karpenter parameters are rejected for GCP, Azure, and DigitalOcean Racks.

> **Disambiguation:** the term "budget" on this page refers exclusively to **Karpenter disruption budgets**, a Kubernetes scheduling primitive that limits how many nodes Karpenter may disrupt simultaneously during consolidation, expiry, or drift cycles. This is unrelated to the **per-app monthly spend cap** introduced in Convox 3.24.6 (see [Budget Caps](/management/budget-caps) for app-level cost controls and the `convox.yml` `budget:` block). The two concepts share a name but operate at different layers (cluster node scheduling vs. application spend) and have no shared configuration surface.

## How Karpenter Works with Convox

When Karpenter is enabled, your Rack's node provisioning is split into three tiers:

| Tier | Managed by | Node type | Purpose |
|------|-----------|-----------|---------|
| **System nodes** | EKS managed node groups (always) | ON_DEMAND | Rack control plane: API server, router, resolver, metrics-server, Karpenter controller |
| **Workload nodes** | Karpenter NodePool | Configurable | Your application Services |
| **Build nodes** | Karpenter NodePool | Configurable | `convox build` / `convox deploy` build pods |

Karpenter replaces Cluster Autoscaler for workload and build node scaling. System nodes always remain as EKS managed node groups to protect the Karpenter controller's own availability. System pods are pinned to system nodes via `nodeSelector` when Karpenter is enabled.

**Karpenter version:** `1.13.1` (pinned, not user-configurable)

### Why Karpenter Over Cluster Autoscaler

Cluster Autoscaler (CAS) works at the Auto Scaling Group (ASG) level. It can only scale groups of identical instances and reacts to pending pods by incrementing ASG desired count. Karpenter works at the pod level, directly evaluating pending pod requirements and provisioning the optimal instance type, size, and purchasing model in seconds rather than minutes.

- **Faster scaling.** Karpenter provisions nodes in response to pending pods within seconds, compared to the multi-minute feedback loop of CAS
- **Cost optimization.** Karpenter selects the cheapest instance type that satisfies pod requirements from across all allowed families and sizes
- **Node consolidation.** Underutilized nodes are automatically consolidated. Karpenter moves pods to fewer, better-utilized nodes and terminates the empty ones
- **Automatic node replacement.** Nodes are replaced after `karpenter_node_expiry` (default 30 days), keeping your fleet on current AMIs
- **Scale-to-zero builds.** The build NodePool scales to zero when no builds are running, eliminating idle build node costs
- **Multi-architecture support.** Workload node architecture is auto-detected from `node_type`, or set explicitly with `karpenter_arch`

## Enabling Karpenter

Most users enable Karpenter with a single command:

```bash
$ convox rack params set karpenter_auth_mode=true karpenter_enabled=true -r rackName
Updating parameters... OK
```

Karpenter uses a two-parameter enablement model:

1. **`karpenter_auth_mode=true`**: A **one-way migration** that prepares the EKS cluster. It migrates the cluster to `API_AND_CONFIG_MAP` access mode and applies `karpenter.sh/discovery` tags to subnets and security groups. This cannot be reversed once enabled (matching AWS EKS behavior).

2. **`karpenter_enabled=true`**: A **bidirectional toggle** that deploys the Karpenter controller, NodePools, IAM roles, and SQS interruption queue. Requires `karpenter_auth_mode=true`. Can be toggled on and off freely.

Both can be set in the same call. If setting them separately, `karpenter_auth_mode` must be set first and the update must complete before setting `karpenter_enabled`.

### Enablement Validation Guards

The CLI validates parameter combinations when enabling Karpenter to prevent scheduling deadlocks and stuck rack updates. These guards run at `convox rack params set` time and reject invalid combinations with actionable error messages.

| Guard | Trigger | Resolution |
|-------|---------|------------|
| `node_capacity_type` must be `ON_DEMAND` | Enabling Karpenter when `node_capacity_type` is `SPOT` or mixed | Set `node_capacity_type=ON_DEMAND` first, wait for the update, then enable Karpenter |
| Cannot change `node_capacity_type` while active | Any `node_capacity_type` change when `karpenter_enabled=true` | Disable Karpenter first, change capacity type, then re-enable |
| Launch template params blocked on non-HA racks | Enabling Karpenter combined with launch template params (`gpu_tag_enable`, `imds_http_tokens`, `imds_http_hop_limit`, `imds_tags_enable`, `ebs_volume_encryption_enabled`, `user_data`, `user_data_url`, `kubelet_registry_pull_qps`, `kubelet_registry_burst`, `key_pair_name`) on racks with `high_availability=false` | Set the launch template params first in a separate call, wait for the update, then enable Karpenter |

**Why these guards exist:**
- Enabling Karpenter with SPOT or mixed capacity types can deadlock node replacement: Karpenter taints the old node group while the replacement may not schedule due to capacity constraints.
- On non-HA racks (single node), combining `karpenter_enabled=true` with launch template changes triggers a rolling update on the only node while Karpenter simultaneously taints it, leaving no schedulable nodes.

All guards can be bypassed with `--force` if you are confident the combination is safe for your specific rack configuration.

### Migrating Workloads to Karpenter Nodes

Once Karpenter is enabled, the system (EKS managed) node group continues running your application workloads alongside core Rack services. To gracefully migrate workloads onto Karpenter-provisioned nodes, set `node_type` to a smaller instance type:

```bash
$ convox rack params set node_type=t3.medium -r rackName
Updating parameters... OK
```

System nodes only need to run core Rack services (API server, router, resolver, Karpenter controller, and pinned add-on controllers). A smaller instance type like `t3.medium` is typically sufficient. Changing `node_type` triggers a rolling update of the managed node group. Kubernetes drains pods off old nodes and Karpenter provisions right-sized workload nodes to absorb them.

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
| `karpenter_arch` | string | _(auto-detect)_ | `amd64`, `arm64`, or `amd64,arm64` | CPU architecture. Unset = auto-detect from `node_type`. Cannot be cleared once set. |
| `karpenter_capacity_types` | string | `on-demand` | `on-demand`, `spot`, or `on-demand,spot` | EC2 purchasing model. When both are set, Karpenter optimizes for cost and falls back to on-demand when spot is unavailable. |

### Architecture Selection and Mixed-Architecture Racks

`karpenter_arch` accepts `amd64`, `arm64`, or `amd64,arm64` (write the value with no spaces). With `amd64,arm64`, Karpenter may provision either architecture and picks the cheapest instance that satisfies each pod's requirements, which often favors arm64 Graviton instances. Once set, `karpenter_arch` cannot be cleared back to auto-detect; set it explicitly to the desired value instead.

#### Architecture Is Inherited, Not Configured Twice

The Karpenter architecture settings derive from the node type parameters a Rack already has, so enabling Karpenter on an existing Rack keeps the architecture that Rack was already running:

| Setting | Derived from |
|:--------|:-------------|
| Workload NodePool architecture | `karpenter_arch` when set, otherwise the architecture of [`node_type`](/configuration/rack-parameters/aws/node_type) |
| Build NodePool architecture | The architecture of [`build_node_type`](/configuration/rack-parameters/aws/build_node_type), which itself falls back to `node_type` |

Leaving `karpenter_arch` unset on an x86 Rack keeps everything amd64, and on a Graviton Rack keeps everything arm64, so a Rack moving to Karpenter does not need to set it. Convox Console Karpenter install templates pre-set the parameters a Karpenter Rack needs, so a Rack installed from a template arrives with a coherent set of values.

#### Build Output

On a Rack with `karpenter_enabled=true`, `karpenter_arch` also determines what a Convox Build produces:

| `karpenter_arch` | Build output |
|------------------|--------------|
| _(unset)_ | One image, native to the build node. |
| A single value (`amd64` or `arm64`) | One image, native to the node the build pod runs on. With [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled) that is a build node, whose architecture follows [`build_node_type`](/configuration/rack-parameters/aws/build_node_type), falling back to [`node_type`](/configuration/rack-parameters/aws/node_type), and does not follow `karpenter_arch`. With the default `build_node_enabled=false` there is no build NodePool, so the build pod runs on a workload node and the image follows `karpenter_arch`. |
| `amd64,arm64` | A multi-architecture image index that runs on either architecture. |

Two things to watch for:

- On a Rack with [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled), setting `karpenter_arch=arm64` by hand on a Rack whose `node_type` is x86 moves the workload pool to arm64 but leaves the build pool on amd64, because the two derive from different parameters. Set [`build_node_type`](/configuration/rack-parameters/aws/build_node_type) to a Graviton type in the same change, or every new Build produces an image the workload pool cannot run and Processes fail with an exec format error. This only arises when you override one side of the derivation; a Rack that inherits both from `node_type` is always consistent. With the default `build_node_enabled=false` there is no build pool, build pods run on workload nodes, and `build_node_type` has no effect.
- An entry in [`additional_karpenter_nodepools_config`](#additional_karpenter_nodepools_config-custom-nodepools) whose `arch` differs from the workload architecture also switches the whole Rack into multi-architecture Builds. On an arm64 Rack, a pool that omits `arch` defaults to `amd64` and does this silently.

Multi-architecture Builds require `karpenter_enabled=true`. Builds run with `--development`, and Builds run with `--external` (which build on your own machine), never receive a platform pin and always produce a single image. Containerized [Resources](/reference/primitives/app/resource) are not architecture-aware and several of the default images have no arm64 variant, so an arm64-capable Rack is not a good fit for them yet.

Scheduling on a mixed-architecture pool follows standard Kubernetes behavior: service pods carry no architecture constraint by default, so a pod can land on a node of either architecture. A multi-architecture image runs on either. A single-architecture image scheduled onto a node of the other architecture fails with an exec format error. Convox system images are multi-arch, and services that reference an external multi-arch image with `image:` can schedule onto either architecture freely.

To pin an individual service to one architecture on a mixed pool, set `nodeSelectorLabels` on the service against the standard architecture label:

```yaml
services:
  web:
    nodeSelectorLabels:
      kubernetes.io/arch: arm64
```

This renders as a required node affinity on the service's deployment, timers, and `convox run` pods, so its processes only schedule onto nodes matching the image's architecture. See [Workload Placement](/configuration/scaling/workload-placement) for the full `nodeSelectorLabels` reference.

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
| `karpenter_disruption_budget_nodes` | string | `10%` | `^((100\|[0-9]{1,2})%\|[0-9]+)$` | Maximum nodes disrupted simultaneously: a node count, or a percentage from `0%` to `100%` (e.g., `10%`, `3`). Percentages above `100%` are rejected. |

### Storage

| Parameter | Type | Default | Validation | Description |
|-----------|------|---------|------------|-------------|
| `karpenter_node_disk` | number | `0` | >= 0 | EBS volume size in GiB for Karpenter-provisioned nodes. `0` inherits the Rack's [`node_disk`](/configuration/rack-parameters/aws/node_disk) value. |
| `karpenter_node_volume_type` | string | `gp3` | `gp2`, `gp3`, `io1`, `io2` | EBS volume type for Karpenter-provisioned nodes. |
| `karpenter_node_os` | string | `al2023` | `al2023`, `bottlerocket` | Node OS for the workload NodePool. `bottlerocket` selects the EKS-optimized Bottlerocket AMI and the two-volume layout it requires (a `gp3` OS volume on `/dev/xvda` and a data volume on `/dev/xvdb`). |

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
| [`karpenter_build_disruption_budget_nodes`](/configuration/rack-parameters/aws/karpenter_build_disruption_budget_nodes) | string | `100%` | `^((100\|[0-9]{1,2})%\|[0-9]+)$` | Maximum empty build nodes reclaimed simultaneously: a node count, or a percentage from `0%` to `100%` (e.g., `100%`, `1`). Drift on the build pool stays capped at a fixed `10%`. |
| [`karpenter_build_imds_tokens`](/configuration/rack-parameters/aws/karpenter_build_imds_tokens) | string | _(none)_ | `optional`, `required` | IMDSv2 token requirement for build nodes. Unset inherits the Rack's [`imds_http_tokens`](/configuration/rack-parameters/aws/imds_http_tokens) value. |
| [`karpenter_build_imds_hop_limit`](/configuration/rack-parameters/aws/karpenter_build_imds_hop_limit) | number | `0` | >= 0 | IMDS response hop limit for build nodes. `0` inherits the Rack's [`imds_http_hop_limit`](/configuration/rack-parameters/aws/imds_http_hop_limit) value. |
| `karpenter_build_node_labels` | string | _(none)_ | Comma-separated `key=value`; no double quotes; `convox-build` and `convox.io/nodepool` reserved | Extra labels added alongside default `convox-build=true` and `convox.io/nodepool=build` labels. |

### Build Node Behavior with Karpenter

When Karpenter is enabled and `build_node_enabled=true`:

- The existing EKS managed build node group is scaled to zero
- Karpenter's build NodePool provisions nodes on-demand when build pods are scheduled
- Build nodes have a `dedicated=build:NoSchedule` taint, so only build pods run on them
- Build nodes scale back to zero after the last build completes. Two parameters govern the turndown: `karpenter_build_consolidate_after` (default `60s`) sets **when** empty build nodes are reclaimed, and [`karpenter_build_disruption_budget_nodes`](/configuration/rack-parameters/aws/karpenter_build_disruption_budget_nodes) (default `100%`) sets **how many** go at once
- Architecture is auto-detected from `build_node_type`
- Build nodes can run under a dedicated least-privilege IAM role by setting [`build_node_minimal_role_enabled`](/configuration/rack-parameters/aws/build_node_minimal_role_enabled), and their IMDS options can be set independently of workload nodes with `karpenter_build_imds_tokens` and `karpenter_build_imds_hop_limit`
- The existing `build_node_min_count` parameter does not apply when Karpenter manages builds

Keep `karpenter_build_instance_families` consistent with the build architecture: restricting build nodes to families of the other architecture (for example `c7g` while `build_node_type` is amd64) leaves no instance type satisfying both constraints, and builds cannot schedule. As of `3.25.3`, `convox rack params set` rejects that combination instead of letting it through, with `karpenter_build_instance_families (c7g) has no family matching the build_node_type architecture (amd64); Karpenter could not provision any matching node`. The same check applies to `karpenter_instance_families` against `karpenter_arch`, and it only runs when `karpenter_enabled=true`. Clear the restriction with `convox rack params set karpenter_build_instance_families=`.

## Advanced Configuration

### `karpenter_config` (Workload NodePool Override)

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
| `ec2NodeClass.amiSelectorTerms` | Custom AMI selection. The default follows [`karpenter_node_os`](/configuration/rack-parameters/aws/karpenter_node_os): `al2023@latest` or `bottlerocket@latest`. An explicit `amiSelectorTerms` overrides it. |
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
Updating parameters... OK
```

### `additional_karpenter_nodepools_config` (Custom NodePools)

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
| `disruption_budget_nodes` | string | `10%` | no | Max nodes disrupted simultaneously: a node count, or a percentage from `0%` to `100%`. Percentages above `100%` are rejected. |
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
Updating parameters... OK
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
Updating parameters... OK
```

### Node Overlays (Fractional GPUs)

A NodeOverlay adjusts how Karpenter simulates a set of instance types without changing the NodePool itself. Overlays can advertise extended resources, override the price Karpenter uses in its cost model, or apply a percentage price adjustment. Configure them with the [`karpenter_node_overlays_config`](/configuration/rack-parameters/aws/karpenter_node_overlays_config) parameter, which also enables the Karpenter `NodeOverlay` feature gate when non-empty.

The main use is fractional-GPU instance families such as `g6f` and `gr6f`. AWS reports these types with a GPU count of zero, so Karpenter prices them but will not provision them for `nvidia.com/gpu` requests. An overlay that advertises `nvidia.com/gpu` on those types lets Karpenter schedule GPU workloads onto them.

**Walkthrough: schedule a GPU service on g6f**

1. Advertise the GPU resource on the g6f/gr6f types with an overlay:

   ```bash
   $ convox rack params set karpenter_node_overlays_config='[{"name":"g6f-fractional-gpu","weight":100,"requirements":[{"key":"node.kubernetes.io/instance-type","operator":"In","values":["g6f.large","g6f.xlarge","g6f.2xlarge","g6f.4xlarge","gr6f.4xlarge"]}],"capacity":{"nvidia.com/gpu":"1"}}]' -r rackName
   Updating parameters... OK
   ```

2. Create a g6f NodePool and enable the device plugin so the advertised GPU is real:

   ```bash
   $ convox rack params set additional_karpenter_nodepools_config='[{"name":"g6f","instance_families":"g6f","capacity_types":"on-demand","cpu_limit":64,"memory_limit_gb":256,"taints":"nvidia.com/gpu=true:NoSchedule","labels":"pool=g6f"}]' nvidia_device_plugin_enable=true -r rackName
   Updating parameters... OK
   ```

3. Target the pool from `convox.yml`. GPU services auto-tolerate the `nvidia.com/gpu` taint:

   ```yaml
   # convox.yml
   services:
     ml-worker:
       build: .
       scale:
         gpu:
           count: 1
           vendor: nvidia
       nodeSelectorLabels:
         convox.io/nodepool: g6f
   ```

> The overlay's `capacity` affects scheduling simulation only. The node still needs the NVIDIA device plugin to advertise a real `nvidia.com/gpu`, and `nvidia_device_time_slicing_replicas` (if set) multiplies fractional GPUs the same way it does full ones.

### Using Taints to Protect Nodes

Without taints, any pod can land on a custom NodePool's nodes if they have spare capacity, even pods that don't need those resources. For example, a basic web service could get scheduled to an expensive GPU instance. Taints prevent this by rejecting pods that lack a matching toleration.

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

When a Service declares `scale.gpu`, Convox adds the GPU extended-resource key (e.g. `nvidia.com/gpu` or `amd.com/gpu`) to the pod's resource requests AND emits a matching `tolerations:` entry (`operator: Exists`, `effect: NoSchedule`) directly in the pod spec. Pods schedule onto tainted GPU nodepools without requiring the Kubernetes `ExtendedResourceToleration` admission controller (which is not enabled by default on EKS).

You must also enable the NVIDIA device plugin on the Rack:

```bash
$ convox rack params set nvidia_device_plugin_enable=true -r rackName
Updating parameters... OK
```

#### Dedicated pools

The simplest way to isolate a pool is `dedicated: true`. This auto-applies a `dedicated-node={name}:NoSchedule` taint and Convox auto-injects the matching toleration for any Service with `nodeSelectorLabels: { convox.io/nodepool: {name} }`. No external webhooks or manual taint configuration needed.

#### Non-GPU custom taints

For custom taints beyond `dedicated` (e.g., tenant isolation with specific taint keys), Convox does not auto-inject tolerations. Pods targeting pools with custom non-GPU taints will need tolerations added through an external mechanism such as a Kubernetes mutating admission webhook. For most non-GPU use cases, using `dedicated: true` or `labels` + `nodeSelectorLabels` without taints is the recommended approach. Karpenter only provisions nodes for pods that need them, so unwanted pods won't cause unnecessary scaling.

#### DaemonSets on tainted nodes

Node-level DaemonSets (fluentd, `aws-node`, `kube-proxy`, `ebs-csi-node`, `efs-csi-node`, `eks-pod-identity-agent`) use `operator: Exists` tolerations and are **not affected** by custom taints. They will continue to run on all nodes, including tainted custom NodePool nodes.

You can also use a JSON file:

```bash
$ convox rack params set additional_karpenter_nodepools_config=/path/to/nodepools.json -r rackName
Updating parameters... OK
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
  - CoreDNS
  - EBS CSI controller
  - EFS CSI controller
  - AWS Load Balancer Controller
- Fluentd DaemonSet is **not** pinned; it runs on all nodes for log collection

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

When disabling Karpenter, set `node_type` to a larger instance alongside `karpenter_enabled=false` so the managed node group can accommodate workloads returning from Karpenter-provisioned nodes:

```bash
$ convox rack params set karpenter_enabled=false node_type=t3.xlarge -r rackName
Updating parameters... OK
```

This triggers the following sequence:

1. Karpenter controller drains workload and build nodes (5-minute graceful drain window)
2. All Karpenter NodePools, EC2NodeClasses, IAM resources, and SQS queue are destroyed
3. The managed node group scales up with the new `node_type` to absorb workloads
4. The managed build node group scales back up from zero to `build_node_min_count`
5. Cluster Autoscaler resumes normal auto-discovery mode
6. System pod `nodeSelector` for `convox.io/system-node` is removed

### Cleaning Up Orphaned Nodes

If Karpenter nodes remain after disabling (for example, due to finalizer deadlocks or interrupted applies), use the cleanup command:

```bash
$ convox rack karpenter cleanup -r rackName
Cleaning up Karpenter nodes... OK
```

This cordons, drains, and deletes any remaining Karpenter-labeled nodes, terminates their backing EC2 instances, and removes stale NodePool and EC2NodeClass CRD objects. Safe to run multiple times.

> `karpenter_auth_mode` cannot be reverted. The EKS access config migration and discovery tags remain. This is safe and has no cost or operational impact. It means the cluster is ready for Karpenter to be re-enabled at any time.

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
- [eks_access_entries](/configuration/rack-parameters/aws/eks_access_entries) for migrating to EKS Access Entries (shares the auth mode switch with `karpenter_auth_mode`)
