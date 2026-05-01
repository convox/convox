---
title: "gpu_observability_enable"
slug: gpu_observability_enable
url: /configuration/rack-parameters/aws/gpu_observability_enable
---

# gpu_observability_enable

## Description
The `gpu_observability_enable` parameter installs the NVIDIA DCGM exporter as a DaemonSet on every GPU node. The exporter emits NVIDIA GPU telemetry — utilization, framebuffer (VRAM), temperature, power — on port 9400.

Enabling this parameter ALSO installs a lightweight free-tier Prometheus (release `prometheus-gpu-metrics` in `kube-system` ns, `prometheus-community/prometheus` chart) scoped to scraping DCGM only. This Prometheus has no Operator, no CRDs, no remote_write — it serves only the rack-API's GPU metric enrichment for `convox ps` and Console GPU dashboards.

**Resource overhead per GPU rack:** ~512Mi memory request, 1024Mi limit; 100m CPU request, 500m limit. Sizing rule-of-thumb: budget 1Gi per 50 concurrent GPU pods.

**If your rack has the Convox metered metrics offering enabled** (the internal flag `monitoring_metrics_provisioned=true`, set automatically by the metered metrics enable/disable flow — not customer-settable), the free chart is suppressed and DCGM is scraped by the metered `kube-prometheus-stack` Prometheus in `convox-monitoring` ns instead. Both paths populate the same GraphQL fields.

## Default Value
The default value for `gpu_observability_enable` is `false`.

## Use Cases
- **GPU job throughput monitoring**: Track per-pod and per-service GPU utilization so you can size your fleet to actual demand rather than guess at peak headroom.
- **VRAM saturation alerting**: Set Prometheus alerts on `DCGM_FI_DEV_FB_USED / DCGM_FI_DEV_FB_TOTAL` to catch out-of-memory crashes before they happen, particularly for inference workloads near framebuffer limits.
- **GPU cost / utilization reporting**: Combine GPU utilization metrics with `cost_tracking_enable` data to surface dollars-per-GPU-hour vs dollars-per-actual-utilization across your services in Console3 dashboards.
- **Capacity planning for time-sliced GPUs**: When `nvidia_device_time_slicing_replicas` is non-zero, DCGM exposes the underlying physical-GPU saturation so you can verify your slicing ratio matches workload behavior.

## Setting Parameters
To enable the DCGM exporter, also enable the NVIDIA device plugin in the same call (the DCGM exporter relies on the device plugin's pod-resources socket for pod-to-GPU attribution):
```bash
$ convox rack params set gpu_observability_enable=true nvidia_device_plugin_enable=true -r rackName
Setting parameters... OK
```

If `nvidia_device_plugin_enable` is already set to `true` on your rack, you can enable observability alone:
```bash
$ convox rack params set gpu_observability_enable=true -r rackName
Setting parameters... OK
```

To disable:
```bash
$ convox rack params set gpu_observability_enable=false -r rackName
Setting parameters... OK
```

Disabling cleanly uninstalls BOTH Helm releases — the DCGM exporter (DaemonSet, Service, RBAC, ConfigMap, ServiceAccount) AND the free-path Prometheus (`prometheus-gpu-metrics` in `kube-system`) are all removed. Both charts install zero CRDs and zero admission webhooks, so there are no orphan resources to clean up after disable.

## Additional Information
- This parameter is AWS-only. The Convox-supported NVIDIA GPU instance families (P3, P4, G4, G5) are EKS-only at this time.
- Enabling this parameter without `nvidia_device_plugin_enable=true` is rejected at the CLI. The DCGM exporter cannot attribute GPU samples to specific pods without the device plugin's pod-resources socket.
- The DCGM exporter pod schedules only on nodes carrying the `convox.io/gpu-vendor=nvidia` label, which the rack controller applies at runtime when a node's instance type is in the NVIDIA GPU list. If you have no GPU nodes in your cluster, the DaemonSet is created but no pods are scheduled — no error, just an empty DaemonSet.
- Scraping happens automatically via the free-path or paid-path in-cluster Prometheus described above. No extra Prometheus deployment required.
- Resource footprint per GPU node: 100m CPU request / 200m CPU limit; 128Mi memory request / 512Mi memory limit. Modest overhead even on dense GPU instance types.

## Related Parameters
- [nvidia_device_plugin_enable](/configuration/rack-parameters/aws/nvidia_device_plugin_enable): Required prerequisite. The DCGM exporter relies on the device plugin's pod-resources socket for pod-to-GPU attribution.
- [nvidia_device_time_slicing_replicas](/configuration/rack-parameters/aws/nvidia_device_time_slicing_replicas): When using time-sliced GPUs, DCGM exposes physical-GPU saturation to help validate your slicing ratio.
- [gpu_observability_chart_version](/configuration/rack-parameters/aws/gpu_observability_chart_version): Pin the DCGM exporter chart version. Use to roll forward to a CVE hotfix without waiting for a Convox release.
- [prometheus_gpu_metrics_chart_version](/configuration/rack-parameters/aws/prometheus_gpu_metrics_chart_version): Pin the chart version for the free-path GPU-metrics Prometheus installed alongside the DCGM exporter.
- [prometheus_gpu_metrics_retention](/configuration/rack-parameters/aws/prometheus_gpu_metrics_retention): Retention window for the free-path GPU-metrics Prometheus.
- [prometheus_url](/configuration/rack-parameters/aws/prometheus_url): Override the rack's auto-resolved Prometheus endpoint with a customer-supplied URL.
- [gpu_tag_enable](/configuration/rack-parameters/aws/gpu_tag_enable): Tags GPU resources for AWS-side cost allocation; complements observability metrics for full GPU cost visibility.

## Version Requirements
This feature requires at least Convox rack version `3.24.6`.
