---
title: "Autoscale Triggers Override"
slug: autoscale-triggers
url: /console/autoscale-triggers
---
# Autoscale Triggers Override

The Console provides a per-Service autoscale configuration that lets you enable, edit, and disable autoscaling through the UI without editing `convox.yml`. Console-driven autoscale settings persist across deploys.

## Autoscale States

Each Service displays one of three autoscale states in its Scaling tab:

| State | Badge | Action |
|---|---|---|
| Not configured | `Autoscale: Not configured` | Enable triggers |
| From `convox.yml` | `Autoscale: From convox.yml` | Override triggers |
| Override active | `Autoscale: Override active` | Disable override |

**Not configured:** The Service has no autoscaling defined in `convox.yml` and no Console override. The Service runs at a fixed replica count.

**From convox.yml:** The Service has autoscaling defined in `convox.yml` (via `scale.autoscale`, `scale.targets`, or `scale.count: N-M`). The Console displays the configured thresholds and offers an Override triggers action.

**Override active:** A Console-driven autoscaler is managing this Service. The `convox.yml` autoscale configuration is preserved but inactive until the override is disabled.

## Trigger Types

| Type | Requirements |
|---|---|
| CPU utilization | Works on every Rack |
| Memory utilization | Works on every Rack |
| GPU utilization | Requires `keda_enable=true` and `scale.gpu.count >= 1` in `convox.yml` |
| Queue depth | Requires `keda_enable=true` |

CPU and memory triggers work on any Rack. GPU and queue depth triggers require [KEDA](/configuration/scaling/keda) to be enabled.

GPU triggers also require the Service to declare GPU resources in `convox.yml`. Without a GPU reservation, the trigger has no metrics to scale on. The Console displays an error if GPU triggers are selected without a GPU reservation.

## Enable / Override / Disable

### Enable Triggers

Click **Enable triggers** on a Service with no autoscale configuration. The dialog asks for:

- **Min replicas** (defaults to current replica count)
- **Max replicas** (defaults to 3x current count or 5, whichever is higher)
- **Trigger checkboxes:** Select the trigger types to enable. Each row has a threshold input with a default value (CPU 70%, Memory 80%, GPU utilization 75%, Queue depth 100).

At least one trigger must be selected to save.

### Override Triggers

Click **Override triggers** on a Service with existing `convox.yml` autoscale settings. The dialog pre-populates with the current thresholds from the manifest. Uncheck a trigger to remove it from the override.

### Edit Thresholds

When an override is active, each threshold cell in the Scaling tab shows a pencil icon. Click to edit the value inline. Changes take effect without re-enabling the full override.

### Disable Override

Click **Disable override** to remove the Console-driven autoscaler. On the next deploy, the `convox.yml` autoscale configuration (if any) takes effect again. If the manifest has no autoscale block, the Service returns to a fixed replica count.

## CLI Commands

The same operations are available from the CLI:

```bash
$ convox services triggers enable web --min 1 --max 5 --cpu 70 -a my-app
$ convox services triggers disable web -a my-app
$ convox services triggers threshold-set web --type cpu --threshold 80 -a my-app
```

The `--type` argument accepts `cpu`, `memory`, `gpu`, or `queue`.

## What Persists Across Deploys

Console-driven autoscale settings persist across deploys. The autoscaler continues running with the same thresholds and replica bounds regardless of new releases.

When the override is disabled, the Console-driven autoscaler is removed and the next deploy restores the `convox.yml` autoscale configuration (or removes autoscaling if the manifest has none).

## Limitations

The Console override covers the four trigger types listed above. Advanced autoscale configuration stays in `convox.yml`:

- Cooldown, polling interval, and stabilization window settings
- Custom KEDA triggers (`scale.keda.triggers` block for SQS, Pub/Sub, custom Prometheus queries)
- Vertical Pod Autoscaler (independent of triggers)

**Important for `scale.keda.triggers` users:** Enabling a Console override replaces any custom KEDA triggers with the four Console-managed trigger types. Custom triggers are restored on the next deploy after you disable the override.

## Authorization

Autoscale trigger actions require the Read+Write role on the App, the same permission level as deploy, scale, and environment edits. Each action is recorded in the audit log with the acting user's email.

## Requirements

Requires Rack version 3.24.6 or later. The Console displays an upgrade-required message when targeting an older Rack.

## Operational Notes

- **Scale to zero:** CPU and memory triggers require `min >= 1`. To enable scale-to-zero, use GPU or queue depth triggers (which require KEDA).
- **Threshold propagation:** After editing a threshold, the new value takes effect within 15 to 30 seconds.

## See Also

- [KEDA Autoscaling](/configuration/scaling/keda)
- [Service Detail](/console/service-detail)
- [Scaling](/configuration/scaling)
