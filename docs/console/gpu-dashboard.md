---
title: "GPU Dashboard"
slug: gpu-dashboard
url: /console/gpu-dashboard
---
# GPU Dashboard

The GPU Dashboard provides real-time telemetry for GPU-accelerated Services running on a Rack. It displays per-Service utilization summaries, time-series charts, and per-Process snapshots sourced from NVIDIA DCGM exporters.

## Prerequisites

- Rack version **3.24.6** or later
- The `gpu_observability_enable` Rack parameter set to `true` (see [GPU Observability](/configuration/rack-parameters/aws/gpu_observability_enable))
- The `nvidia_device_plugin_enable` Rack parameter set to `true`
- GPU telemetry scraper enabled in Rack Settings (Console toggle)
- At least one Service with a `gpu` block in its `convox.yml`

## Accessing the Dashboard

Navigate to **Organization > Rack > App > GPU Telemetry** in the Console. The dashboard appears as a tab on the App detail page.

## Setup Flow

The dashboard guides you through setup with contextual banners:

1. **GPU observability disabled:** If `gpu_observability_enable` is `false`, a banner links to Rack Settings with the CLI command to enable it.
2. **No GPU Services:** If the App has no Services with a `gpu` block in `convox.yml`, the dashboard shows example manifests for basic GPU, vendor-specific, and autoscaling configurations.
3. **Scraper not enabled:** If the DCGM exporter is running but the Console telemetry scraper has not been toggled on, a banner links to Rack Settings.
4. **Awaiting telemetry:** After enabling, metrics appear within 30-90 seconds as the exporter completes its first collection cycle.

## Per-Service Summary Cards

Each GPU Service displays a summary card showing:

- **GPU utilization:** Mean utilization across all pods, displayed as a percentage.
- **GPU memory:** Used vs. total VRAM in bytes.
- **Tensor core active:** Percentage of tensor core utilization (key efficiency metric for inference workloads).
- **SM active:** Streaming multiprocessor activity percentage.
- **DRAM active:** Device memory bandwidth utilization.
- **Power draw:** Current power consumption in watts.
- **FP16 / FP32:** Floating-point precision utilization.

Hover over the utilization number to see how many GPU pods contribute to the mean.

## Time-Series Charts

Four charts display historical metrics per Service:

| Chart | Y-axis | Description |
|---|---|---|
| GPU Utilization | 0-100% | Overall GPU compute usage |
| GPU Memory Used | bytes | VRAM consumption over time |
| Tensor Core Active | 0-100% | Tensor operation throughput |
| Power Draw | watts | Energy consumption |

Each chart shows one line per GPU Service. Null values from pod restarts render as gaps rather than interpolated lines.

### Time Range Selection

Select a display window from the dropdown in the dashboard header:

| Window | Tick interval | Use case |
|---|---|---|
| 5 min | 30 seconds | Real-time debugging |
| 30 min | 5 minutes | Short-term trend |
| 1 hour | 10 minutes | Default view |
| 24 hour | 1 hour | Daily patterns |

The selection persists across browser tabs and page navigations via localStorage. Changing the window in one tab updates all open GPU Dashboard tabs on the same browser.

## Per-Process Snapshot Table

Below the charts, a table lists every GPU Process with columns for Process ID, Service, Status, GPU %, GPU Memory, Tensor %, SM %, DRAM %, and Power. This table reflects the most recent DCGM scrape (polled every 30 seconds).

## Partial Data Banner

When more than 25% of settled GPU pods (alive for 90+ seconds) report null utilization, a warning banner indicates partial data. This clears automatically once all pods report metrics.

## DCGM Exporter Status Badge

The dashboard header displays a status badge showing the DCGM exporter state:

- **Enabled** (green): Active and reporting metrics.
- **Enabled - waiting for GPU node** (blue): Installed but no GPU nodes are scheduled yet. This is normal with Karpenter before a GPU workload triggers node provisioning.
- **Disabled** (grey): Exporter is off.

Click the badge to navigate to Rack Settings where the exporter and scraper can be configured.

## Grafana Deep Links

If the `grafana_url` Rack parameter is set, each Service summary card displays a Grafana icon that opens the corresponding dashboard in your Grafana instance with Rack, namespace, Service, and App template variables pre-filled.

The template variable names default to `rack`, `namespace`, `service`, and `app`. If your imported dashboards use different variable names, configure the mapping with Rack parameters:

```bash
$ convox rack params set grafana_dashboard_var_rack=cluster_name
$ convox rack params set grafana_dashboard_var_namespace=ns
```

A "Dashboard filter mismatch?" link below the Grafana button opens a troubleshooting modal with the full configuration guidance.

## See Also

- [GPU Metrics (Rack-side setup)](/observability/gpu-metrics)
- [GPU Observability Rack Parameter](/configuration/rack-parameters/aws/gpu_observability_enable)
- [Manifest GPU Configuration](/reference/primitives/app/service#scalegpu)
- [Workload Placement](/configuration/scaling/workload-placement)
- [Service Detail](/console/service-detail)
