---
title: "gpu_observability_enable"
slug: gpu_observability_enable
url: /configuration/rack-parameters/aws/gpu_observability_enable
---

# gpu_observability_enable

## Description
Enable GPU observability infrastructure (DCGM exporter) on this rack. DCGM is a per-node DaemonSet that exports GPU metrics (utilization, memory, temperature) on port 9400.

To consume these metrics: enable Convox Console monitoring (free or paid plan) which deploys a Prometheus chart that scrapes DCGM. Without Console monitoring enabled, DCGM emits metrics with no scraper.

**Resource overhead per GPU rack:** the DCGM exporter itself is modest — 100m CPU request / 200m limit; 128Mi memory request / 512Mi limit. The Console-deployed Prometheus chart adds its own footprint (sized at chart-time per plan).

## Default Value
The default value for `gpu_observability_enable` is `false`.

## Use Cases
- **GPU job throughput monitoring**: Track per-pod and per-service GPU utilization so you can size your fleet to actual demand rather than guess at peak headroom.
- **VRAM saturation alerting**: Set Prometheus alerts on `DCGM_FI_DEV_FB_USED / (DCGM_FI_DEV_FB_USED + DCGM_FI_DEV_FB_FREE + DCGM_FI_DEV_FB_RESERVED)` to catch out-of-memory crashes before they happen, particularly for inference workloads near framebuffer limits. (DCGM's default counters file emits FB_USED, FB_FREE, and FB_RESERVED separately; total framebuffer is the sum of the three.)
- **GPU cost / utilization reporting**: Combine GPU utilization metrics with `cost_tracking_enable` data to surface dollars-per-GPU-hour vs dollars-per-actual-utilization across your services in the Convox Console dashboards.
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

Disabling cleanly uninstalls the DCGM exporter Helm release (DaemonSet, Service, RBAC, ConfigMap, ServiceAccount). The chart installs zero CRDs and zero admission webhooks, so there are no orphan resources to clean up after disable. Console-deployed Prometheus charts are managed independently via the Convox Console — disable monitoring in the Console to remove them.

## Additional Information
- This parameter is AWS-only. The Convox-supported NVIDIA GPU instance families (P3, P4, G4, G5) are EKS-only at this time.
- Enabling this parameter without `nvidia_device_plugin_enable=true` is rejected at the CLI. The DCGM exporter cannot attribute GPU samples to specific pods without the device plugin's pod-resources socket.
- The DCGM exporter pod schedules only on nodes carrying the `convox.io/gpu-vendor=nvidia` label, which the rack controller applies at runtime when a node's instance type is in the NVIDIA GPU list. If you have no GPU nodes in your cluster, the DaemonSet is created but no pods are scheduled — no error, the DaemonSet stays empty.
- Scraping requires a Prometheus chart deployed via the Convox Console (free or paid plan). Enable monitoring in the Console to install the chart that scrapes DCGM. Without Console monitoring, DCGM emits metrics with no scraper and user dashboards stay empty. See [`prometheus_url`](/configuration/rack-parameters/aws/prometheus_url) to wire `convox ps` GPU enrichment to the Console-deployed Prometheus.
- Resource footprint per GPU node: 100m CPU request / 200m CPU limit; 128Mi memory request / 512Mi memory limit. Modest overhead even on dense GPU instance types.
- **Capacity considerations for the Console-deployed Prometheus chart**: enabling monitoring through the Convox Console installs either a lightweight `prometheus-community/prometheus` chart (free plan, kube-system namespace) or a kube-prometheus-stack chart (paid plan, convox-monitoring namespace) on top of the DCGM DaemonSet. The chart's combined steady-state footprint is roughly 1 vCPU and 2 GiB of memory for the paid plan; transient install-time spikes can be 1.5x that. Racks with only a single small workload node (e.g. `t3.small`, `t3.medium`) can overcommit the node and trigger a kubelet failure when the chart installs alongside user workloads. Recommended minimums for racks that intend to surface GPU data through the Console: one node of `t3.large` or larger; two or more workload nodes of any size; or Karpenter enabled (`karpenter_enable=true`) so the rack can grow capacity on demand. Convox 3.24.6 ships explicit resource requests on the prometheus statefulset and a PodDisruptionBudget so the autoscaler pre-provisions a fitting node before scheduling.
- For setup walkthrough, dashboard usage, and troubleshooting, see [GPU observability](/observability/gpu-metrics).

## Related Parameters
- [nvidia_device_plugin_enable](/configuration/rack-parameters/aws/nvidia_device_plugin_enable): Required prerequisite. The DCGM exporter relies on the device plugin's pod-resources socket for pod-to-GPU attribution.
- [nvidia_device_time_slicing_replicas](/configuration/rack-parameters/aws/nvidia_device_time_slicing_replicas): When using time-sliced GPUs, DCGM exposes physical-GPU saturation to help validate your slicing ratio.
- [gpu_observability_chart_version](/configuration/rack-parameters/aws/gpu_observability_chart_version): Pin the DCGM exporter chart version. Use to roll forward to a CVE hotfix without waiting for a Convox release.
- [prometheus_gpu_metrics_chart_version](/configuration/rack-parameters/aws/prometheus_gpu_metrics_chart_version): Pin the chart version for the free-plan Prometheus chart deployed via the Convox Console.
- [prometheus_gpu_metrics_retention](/configuration/rack-parameters/aws/prometheus_gpu_metrics_retention): Retention window for the free-plan Prometheus chart deployed via the Convox Console.
- [prometheus_url](/configuration/rack-parameters/aws/prometheus_url): Set the Prometheus URL for KEDA autoscale and `convox ps` GPU enrichment.
- [gpu_tag_enable](/configuration/rack-parameters/aws/gpu_tag_enable): Tags GPU resources for AWS-side cost allocation; complements observability metrics for full GPU cost visibility.

## Version Requirements
This feature requires at least Convox rack version `3.24.6`.
