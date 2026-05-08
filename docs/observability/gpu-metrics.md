---
title: "GPU observability"
slug: gpu-metrics
url: /observability/gpu-metrics
---

# GPU observability

GPU observability gives you per-pod, per-service, and per-app GPU
telemetry: utilization, memory used and total, tensor / SM / DRAM
activity, FP16 / FP32 throughput, and power draw. Additional DCGM
metrics (clock rates, PCIe throughput, NVLink, throttle reasons) are
collected by the DCGM exporter and visible in Grafana — the Console
dashboard surfaces the 9-field summary above as the everyday
observability view. The Convox Console renders these metrics on the
per-app GPU dashboard with a configurable time-range dropdown; the
same fields are exposed on the rack API for SDK consumers.

This page covers setup, dashboard usage, the four empty-state cases
you may encounter, and troubleshooting steps when telemetry doesn't
appear.

## Terms

This guide uses several Kubernetes and observability concepts. Quick
definitions:

- **DCGM (Data Center GPU Manager)** — NVIDIA's tool for reading GPU
  statistics. Convox installs the upstream `dcgm-exporter` Helm chart
  on GPU nodes when you enable observability.
- **Prometheus** — A metrics storage system that scrapes DCGM at a
  configurable interval and stores the time-series data.
- **Helm** — Kubernetes package manager. Convox uses it to install the
  DCGM exporter chart and (optionally) a free in-cluster Prometheus
  chart.
- **DaemonSet** — A Kubernetes object that runs one copy of a pod on
  each node. The DCGM exporter runs as a DaemonSet so every GPU node
  gets a scrape target.
- **ConfigMap** — A Kubernetes object that holds configuration data.
  The DCGM scrape config and the rack's webhook-receiver config both
  live in ConfigMaps.
- **Scrape interval** — How frequently Prometheus polls DCGM for fresh
  metrics. Lower values give finer time resolution at higher storage
  cost; higher values reduce storage at the cost of resolution.
- **NVIDIA driver** — The host-side driver that DCGM relies on for GPU
  introspection. Convox-supported AMIs ship a compatible driver
  version; if you use a custom AMI, verify driver compatibility with
  the chart version pinned by `gpu_observability_chart_version`.

## Setup

GPU observability is opt-in via two rack params:

```bash
$ convox rack params set gpu_observability_enable=true \
    nvidia_device_plugin_enable=true -r myrack
Setting parameters... OK
```

The `nvidia_device_plugin_enable=true` prerequisite is required: the
DCGM exporter relies on the device plugin's pod-resources socket for
pod-to-GPU attribution. Setting `gpu_observability_enable=true` without
the device plugin is rejected at the CLI.

Once enabled, the rack installs:

- The DCGM exporter as a DaemonSet on GPU nodes (NVIDIA-only;
  AWS-only for v1 across the P3, P4, G4, and G5 EC2 instance
  families).
- A Prometheus scrape target. If Convox metered metrics is enabled,
  the metered Prometheus scrapes DCGM. Otherwise, a free in-cluster
  Prometheus chart auto-installs in the `kube-system` namespace and
  scrapes DCGM.
- Configuration plumbing on the api Deployment so the Convox Console
  can render dashboards from the configured Prometheus endpoint.

### Configuring scrape interval

The default DCGM scrape interval is `15s`. If your Prometheus retention
or storage is cost-sensitive, raise the interval:

```bash
$ convox rack params set dcgm_scrape_interval=30s -r myrack
Setting parameters... OK
```

Allowed values are Go-format duration strings between `5s` and `5m`.
See [`dcgm_scrape_interval`](/configuration/rack-parameters/aws/dcgm_scrape_interval)
for details and the trade-off considerations.

### Redeploying services that use GPU autoscale

Services that use `scale.autoscale.gpu_utilization` or
`scale.autoscale.queue_depth` triggers MUST be redeployed after
enabling observability for the first time. Existing ScaledObjects on
these services have a stale Prometheus URL baked in; the redeploy
refreshes the ScaledObject with the live URL.

```bash
$ convox deploy -a myapp -r myrack
```

CPU- and memory-based triggers, services with an explicit
`prometheusUrl` set on the autoscale block, and `scale.keda.triggers`
custom triggers are NOT affected and need no redeploy.

## What you'll see in the Console

The Convox Console renders GPU telemetry on the per-app GPU dashboard.
Open the app, select the GPU tab.

### Display window dropdown

Above the chart, the **Display window** dropdown lets you pick the
time range:

- `5 min` — recent activity, fine resolution
- `30 min` — recent trend
- `1 hour` — typical operational window
- `24 hour` — daily / overnight pattern

Changing the dropdown refetches data from the rack via the
`App.metricsByService(interval:, services:)` GraphQL field. Chart
history is server-driven via Prometheus range queries — no
client-side buffering needed.

### Summary cards

Each GPU service shows a summary card with the most recent scrape
values: GPU utilization, memory used, memory total, tensor active,
SM active, DRAM active, FP16 / FP32, and power draw. The summary card
values are spatial averages across the service's GPU pods at the most
recent scrape — they are NOT temporal averages.

For DCGM metrics not surfaced in the summary card — clock rates, PCIe
throughput, NVLink, throttle reasons — open the Grafana deep-link
from the dashboard to see the full DCGM panel set.

### Per-service chart

Below the cards, the time-series chart plots utilization for each
service. Use the dropdown to switch the time range. Hover over the
chart to inspect individual values.

### Per-pod detail

In the service detail page, the pod list includes the GPU columns:
utilization, memory used, memory total. Pods that are warming up
(joined within the past 90 seconds) show as warming and are excluded
from partial-state banner calculations.

## Empty states

You may see one of four empty states on the GPU dashboard.

### A. Observability is not enabled

The chart and summary cards are blank. The dashboard shows a banner:
"GPU observability is not enabled on this rack." Run
`convox rack params set gpu_observability_enable=true
nvidia_device_plugin_enable=true` to enable.

### B. No GPU services in this app

GPU observability is enabled but the app has no service with
`scale.gpu.count > 0` configured in `convox.yml`. The dashboard
shows: "This app has no GPU services. Set `scale.gpu.count` in
`convox.yml` to schedule pods on GPU nodes."

### C. Telemetry warming up

Observability is enabled and GPU services are deployed, but the most
recent scrape has not produced data yet. Typical delay is 30-90s
after first deploy. The dashboard shows: "Telemetry is warming up.
Metrics should appear within 90 seconds." Refresh the page after the
delay.

### Partial-state banner (over 25% of pods missing)

If more than 25% of a service's GPU pods are missing utilization data
in the current scrape window, the dashboard shows a partial-state
banner identifying the affected pods. Pods that joined within the
past 90 seconds are not counted against the 25% threshold (warming
up). The banner does NOT block chart render; it informs you that the
displayed averages exclude the affected pods.

Common causes of pods missing utilization:
- The DCGM exporter pod on the affected node crashed or has not
  started.
- The NVIDIA device plugin is in a degraded state on the affected
  node.
- The GPU is in a non-reporting state (e.g., XID error).

See [Troubleshooting](#troubleshooting) below.

## Troubleshooting

### Check DCGM exporter pods

```bash
$ kubectl get pods -n kube-system -l app.kubernetes.io/name=dcgm-exporter
NAME                  READY   STATUS    RESTARTS   AGE
dcgm-exporter-abc12   1/1     Running   0          5m
dcgm-exporter-def34   1/1     Running   0          5m
```

You should see one DCGM exporter pod per GPU node, all in the
`Running` state. If a pod is in `CrashLoopBackOff` or `Pending`,
`kubectl describe pod -n kube-system <pod>` for details. Common
causes: NVIDIA driver mismatch (pin
`gpu_observability_chart_version` to a compatible chart), missing
NVIDIA device plugin on the node, or the node is not labeled with
`convox.io/gpu-vendor=nvidia`.

### Verify Prometheus is scraping DCGM

If you're using the free in-cluster Prometheus:

```bash
$ kubectl port-forward -n kube-system svc/prometheus-gpu-metrics-server 9090:80
```

Open http://localhost:9090/targets and look for the `dcgm-exporter`
job. All targets should show `UP`.

If you're using Convox metered metrics, open the metered Grafana and
inspect the DCGM data source.

### Verify rack params

```bash
$ convox rack params -r myrack | grep -E 'gpu_observability|dcgm|gpu_metrics'
dcgm_scrape_interval         15s
gpu_metrics_max_pods         100
gpu_metrics_max_concurrent   10
gpu_observability_enable     true
```

The defaults are sensible for most workloads. Adjust
`gpu_metrics_max_pods` only if you regularly run more than 100 GPU
pods per service in one app. Adjust `gpu_metrics_max_concurrent` only
if you observe rack-side handler saturation under bursty dashboard
traffic.

### Verifying webhook delivery

If you've configured webhooks for budget caps, auto-shutdown
lifecycle, or release-watcher events and want to confirm delivery:

- Check the api-pod stdout for structured `webhook_dispatch` audit
  log lines:

  ```bash
  $ kubectl logs -n convox-system deployment/api -c api | grep webhook_dispatch
  ```

  Successful dispatches log `audit_type=webhook_dispatch_succeeded
  url_host=hooks.slack.com status=200`. Failed dispatches log
  `audit_type=webhook_dispatch_failed url_host=... status=...`. The
  rack does NOT retry failed dispatches — receivers MUST be
  idempotent and tolerate occasional drops on transient receiver
  errors.

- The webhook ConfigMap supports a JSON receiver-config form to set
  per-receiver delivery timeouts. See
  [Webhooks](/configuration/webhooks#webhook-delivery-hardening) for
  the JSON schema and rotation guidance.

### Open in Grafana deep-link

The Console's per-app GPU dashboard includes an `Open in Grafana`
deep-link button that builds a URL with the dashboard variables
substituted from your rack config. If your Grafana dashboard expects
different variable names than `app`, `service`, `namespace`, or
`rack`, use the `grafana_dashboard_var_*` rack params:

```bash
$ convox rack params set grafana_dashboard_var_app=application -r myrack
$ convox rack params set grafana_dashboard_var_service=svc -r myrack
```

See
[`grafana_dashboard_var_app`](/configuration/rack-parameters/aws/grafana_dashboard_var_app)
and its companions for details. The Console also exposes a
"Dashboard filter mismatch?" troubleshoot modal that explains the four
configurable var names.

## Provider scope

GPU observability ships AWS-only for v1. On GCP / Azure / DigitalOcean
/ Equinix Metal / Local racks, the GPU dashboard renders the
empty-state guidance and the new `gpu-util` / `gpu-mem-used` /
`gpu-mem-total` fields on `Process` and `Service` populate with zero
values. Per-provider backend chains are tracked for subsequent
releases.

## Related parameters

- [`gpu_observability_enable`](/configuration/rack-parameters/aws/gpu_observability_enable):
  Enable the DCGM exporter Helm release.
- [`nvidia_device_plugin_enable`](/configuration/rack-parameters/aws/nvidia_device_plugin_enable):
  Required prerequisite for DCGM.
- [`dcgm_scrape_interval`](/configuration/rack-parameters/aws/dcgm_scrape_interval):
  Tune the DCGM scrape frequency.
- [`gpu_metrics_max_pods`](/configuration/rack-parameters/aws/gpu_metrics_max_pods):
  Per-service pod cap for the metrics handler.
- [`gpu_metrics_max_concurrent`](/configuration/rack-parameters/aws/gpu_metrics_max_concurrent):
  Concurrency cap for the metrics handler.
- [`gpu_observability_chart_version`](/configuration/rack-parameters/aws/gpu_observability_chart_version):
  Pin the DCGM exporter chart version.
- [`prometheus_gpu_metrics_chart_version`](/configuration/rack-parameters/aws/prometheus_gpu_metrics_chart_version):
  Pin the free in-cluster Prometheus chart version.
- [`prometheus_gpu_metrics_retention`](/configuration/rack-parameters/aws/prometheus_gpu_metrics_retention):
  Retention window for the free in-cluster Prometheus chart.
- [`prometheus_url`](/configuration/rack-parameters/aws/prometheus_url):
  Override the Prometheus endpoint used by `convox ps` GPU enrichment
  and KEDA Prometheus-backed autoscale triggers.
- [`grafana_dashboard_var_app`](/configuration/rack-parameters/aws/grafana_dashboard_var_app),
  [`_namespace`](/configuration/rack-parameters/aws/grafana_dashboard_var_namespace),
  [`_rack`](/configuration/rack-parameters/aws/grafana_dashboard_var_rack),
  [`_service`](/configuration/rack-parameters/aws/grafana_dashboard_var_service):
  Override Grafana template variable names for the deep-link button.

## Version requirements

GPU observability requires Convox rack version `3.24.6` or later. The
backend chain (DCGM exporter installed via Helm, Prometheus scrape
wired into rack-side metrics handlers) is AWS-only for v1.
