---
title: "gpu_observability_enable"
slug: gpu_observability_enable
url: /configuration/rack-parameters/aws/gpu_observability_enable
---

# gpu_observability_enable

## Description
The `gpu_observability_enable` parameter installs the NVIDIA DCGM (Data Center GPU Manager) exporter as a DaemonSet on every GPU node in your cluster. The exporter emits NVIDIA GPU telemetry — utilization, framebuffer (VRAM) usage, temperature, power draw — on port 9400 in Prometheus exposition format. The DaemonSet is annotated with `prometheus.io/scrape=true` so the in-cluster Prometheus scrapes it automatically using the kubernetes_sd Pod-role discovery already configured for KEDA.

Enabling this parameter is the first step in turning on Plan-5 GPU observability surfacing — once metrics are flowing, the rack-side enrichment path populates `convox ps -j` and `convox services -j` JSON output with `gpu-util`, `gpu-mem-used`, and `gpu-mem-total` fields, which then surface in Console3 GPU dashboards.

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

Disabling cleanly uninstalls the Helm release — the DaemonSet, Service, RBAC, ConfigMap, and ServiceAccount are all removed. The DCGM exporter chart installs zero CRDs and zero admission webhooks, so there are no orphan resources to clean up after disable.

## Additional Information
- This parameter is AWS-only. The Convox-supported NVIDIA GPU instance families (P3, P4, G4, G5) are EKS-only at this time.
- Enabling this parameter without `nvidia_device_plugin_enable=true` is rejected at the CLI. The DCGM exporter cannot attribute GPU samples to specific pods without the device plugin's pod-resources socket.
- The DCGM exporter pod schedules only on nodes carrying the `convox.io/gpu-vendor=nvidia` label, which the rack controller applies at runtime when a node's instance type is in the NVIDIA GPU list. If you have no GPU nodes in your cluster, the DaemonSet is created but no pods are scheduled — no error, just an empty DaemonSet.
- The exporter reads `prometheus_url` from your rack parameters when configured, but in the default case scraping happens via the in-cluster Prometheus that the rack already runs for KEDA-driven autoscale. No extra Prometheus deployment is required.
- Resource footprint per GPU node: 100m CPU request / 200m CPU limit; 128Mi memory request / 512Mi memory limit. Modest overhead even on dense GPU instance types.

## Related Parameters
- [nvidia_device_plugin_enable](/configuration/rack-parameters/aws/nvidia_device_plugin_enable): Required prerequisite. The DCGM exporter relies on the device plugin's pod-resources socket for pod-to-GPU attribution.
- [nvidia_device_time_slicing_replicas](/configuration/rack-parameters/aws/nvidia_device_time_slicing_replicas): When using time-sliced GPUs, DCGM exposes physical-GPU saturation to help validate your slicing ratio.
- [gpu_observability_chart_version](/configuration/rack-parameters/aws/gpu_observability_chart_version): Pin the DCGM exporter chart version. Use to roll forward to a CVE hotfix without waiting for a Convox release.
- [gpu_tag_enable](/configuration/rack-parameters/aws/gpu_tag_enable): Tags GPU resources for AWS-side cost allocation; complements observability metrics for full GPU cost visibility.

## Version Requirements
This feature requires at least Convox rack version `3.24.6`.
