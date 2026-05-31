---
title: "Rack Settings"
slug: rack-settings
url: /console/rack-settings
---
# Rack Settings

The Rack Settings page in the Console provides configuration controls for Rack-wide features. Navigate to it from the sidebar by selecting a Rack and clicking **Settings**.

## Role Badge

A badge in the Rack header shows your effective role on the current Rack:

| Badge | Access Level |
|-------|-------------|
| Read-only | View Rack, Apps, and settings. Cannot modify. |
| Read+Write | Full operational access (deploy, scale, configure). |
| Administrator | All permissions including member management and destructive actions. |

The badge links to the role legend page for your organization. Pre-3.24.6 Racks do not display a role badge.

## Cost Tracking

The Cost Tracking card shows whether per-App spend accumulation is enabled on the Rack. Cost tracking powers budget caps, alerts, and auto-shutdown features.

**Current status** is displayed as an Enabled or Disabled badge.

**To toggle**, run the CLI command shown on the card:

```bash
$ convox rack params set cost_tracking_enable=true -r <rack-name>
```

Cost tracking requires Rack version 3.24.6 or later. The card displays a version gate message on older Racks.

See [Cost Tracking](/management/cost-tracking) for setup and usage details.

## GPU Telemetry Scraper

The GPU Telemetry Scraper card controls the Prometheus scraper that collects GPU metrics from DCGM exporters on each node.

**Prerequisites:**
- The DCGM Exporter (`gpu_observability_enable`) must be enabled first. The scraper toggle is locked until DCGM is active.
- Rack version 3.24.6 or later.

**Toggle** the scraper on or off directly from the card. The toggle responds immediately and stays interactive during transitions. Status states:

| Status | Meaning |
|--------|---------|
| Enabled | Scraper is running and collecting metrics |
| Enabling... | Rack update in progress to install the scraper |
| Disabled | Scraper is not installed |
| Disabling... | Rack update in progress to remove the scraper |
| Error: ... | Worker failure; admin reset may be available |

**Admin Reset:** When the scraper enters a frozen error state, administrators see a Reset link to clear it. The toggle remains interactive so you can request a new state regardless of the current error.

Toggling applies a non-disruptive Rack update. Running workloads are not affected.

See [GPU Metrics](/observability/gpu-metrics) for Rack-side setup and [GPU Dashboard](/console/gpu-dashboard) for the Console monitoring view.

## KEDA Status

The KEDA card shows whether Kubernetes Event-Driven Autoscaling is enabled on the Rack. KEDA enables GPU utilization, inference queue depth, custom Prometheus, and message-queue autoscale triggers. CPU and memory autoscaling works without KEDA.

**To toggle**, run the CLI command shown on the card:

```bash
$ convox rack params set keda_enable=true -r <rack-name>
```

The card includes expandable examples for:
- Classic autoscale (CPU/memory targets, no KEDA required)
- KEDA-based autoscale (GPU, queue depth, CPU thresholds)
- Direct KEDA scaler (raw KEDA trigger config in convox.yml)

See [KEDA Autoscaling](/configuration/scaling/keda) for full documentation.

## Webhook Signing Key Management

The Webhook Signing Key section manages the HMAC-SHA256 key used to sign outbound webhooks. Receivers verify the signature header (`Convox-Signature: t=...,v1=...`).

**Actions (Administrator only):**

- **Generate first key:** Creates the initial signing key when none exists.
- **Reveal:** Shows the current key value (auto-masked after display).
- **Rotate:** Generates a new key. Up to 4 keys remain active during rotation so receivers can transition. The oldest key is evicted when the limit is reached.

A rotation-in-progress banner appears when multiple keys are active, prompting you to update receivers and confirm the rotation.

Non-administrator users see a message indicating that administrator access is required.

See [Webhook Signing](/console/webhook-signing) for receiver verification instructions.

## Rack Uninstall

The Uninstall Rack card is a destructive action that permanently removes all Apps, resources, and infrastructure associated with the Rack. This action cannot be undone.

Click **Uninstall Rack** to open a confirmation dialog. The uninstall process tears down all Terraform-managed infrastructure for the Rack.

## See Also

- [CLI Rack Management](/management/cli-rack-management) for managing Racks from the command line
- [Console Rack Management](/management/console-rack-management) for moving Racks between CLI and Console
- [RBAC](/management/rbac) for configuring organization access controls
- [Autoscale Triggers](/console/autoscale-triggers) for Console-driven autoscaling
- [GPU Dashboard](/console/gpu-dashboard) for GPU telemetry visualization
- [Budget Management](/console/budget-management) for per-App spend tracking and caps
