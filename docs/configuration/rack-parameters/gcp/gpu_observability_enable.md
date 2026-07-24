---
title: "gpu_observability_enable"
description: "The gpu_observability_enable GCP rack parameter installs the NVIDIA DCGM exporter to export GPU metrics like utilization, memory, and temperature on GKE GPU nodes."
slug: gpu_observability_enable
url: /configuration/rack-parameters/gcp/gpu_observability_enable
---

# gpu_observability_enable

## Description
Enable GPU observability infrastructure (DCGM exporter) on this rack. DCGM is a per-node workload that exports GPU metrics (utilization, memory, temperature) on port 9400.

On GKE the NVIDIA device plugin and drivers are managed by the platform, so no separate device-plugin parameter is required. The exporter exposes metrics with `prometheus.io/*` pod annotations. To consume them, point a Prometheus you run yourself, or Google Managed Prometheus, at the exporter. GCP racks do not deploy a Convox-managed Prometheus.

**Resource overhead per GPU node:** 100m CPU request / 200m limit and 128Mi memory request / 512Mi limit.

## Default Value
The default value for `gpu_observability_enable` is `false`.

## Use Cases
- **GPU job throughput monitoring**: Track per-pod and per-service GPU utilization so you can size your fleet to actual demand.
- **VRAM saturation alerting**: Set Prometheus alerts on `DCGM_FI_DEV_FB_USED / (DCGM_FI_DEV_FB_USED + DCGM_FI_DEV_FB_FREE + DCGM_FI_DEV_FB_RESERVED)` to catch out-of-memory crashes before they happen.
- **GPU cost / utilization reporting**: Combine GPU utilization metrics with cost data to surface dollars-per-GPU-hour vs dollars-per-actual-utilization.

## Setting Parameters
To enable the DCGM exporter:
```bash
$ convox rack params set gpu_observability_enable=true -r rackName
Updating parameters... OK
```

To disable:
```bash
$ convox rack params set gpu_observability_enable=false -r rackName
Updating parameters... OK
```

Disabling cleanly uninstalls the DCGM exporter Helm release (DaemonSet, Service, RBAC, ConfigMap, ServiceAccount) and the GPU dashboard ConfigMaps. The chart installs zero CRDs and zero admission webhooks, so there are no orphan resources to clean up.

## Downgrading

Disabling on the current version (above) is always safe. If you plan to downgrade the rack to a version earlier than the one that introduced GPU observability (`3.25.3`), first set `gpu_observability_enable=false` and let that update complete, then downgrade. This removes the DCGM Helm release while the rack still knows how to manage it. Downgrading while the exporter is still enabled can strand the Helm release in Terraform state and block further updates.

## Additional Information
- GKE manages the NVIDIA device plugin and drivers for GPU node pools, so there is no `nvidia_device_plugin_enable` parameter on GCP. Enabling observability is a single switch.
- The DCGM exporter pod schedules only on nodes carrying the `convox.io/gpu-vendor=nvidia` label, which the rack controller applies at runtime when a node's machine type is a GPU family (`g2-`, `a2-`, `a3-`, `a4-`, `a4x-`, `g4-`). If you have no GPU nodes, the exporter is created but no pods are scheduled.
- Enabling observability also installs GPU Grafana dashboard ConfigMaps in `kube-system`, labeled `grafana_dashboard=1` for discovery by a bring-your-own Grafana sidecar.
- `convox ps` GPU enrichment and the Console GPU dashboards are AWS-only for now. GCP racks do not deploy a Convox-managed Prometheus and do not wire `prometheus_url`; point your own Prometheus/Grafana at the exporter instead.

> **N1 machine types are not auto-detected.** GPU nodes are identified by machine family (`g2-`, `a2-`, `a3-`, `a4-`, `a4x-`, `g4-`). N1 machines with attached GPUs cannot be detected by machine type alone and will not receive the `convox.io/gpu-vendor=nvidia` label, so the DCGM exporter will not schedule onto them. Prefer the dedicated GPU families, or label such nodes manually.

## Related Parameters
- [gpu_observability_chart_version](/configuration/rack-parameters/gcp/gpu_observability_chart_version): Pin the DCGM exporter chart version.
- [dcgm_scrape_interval](/configuration/rack-parameters/gcp/dcgm_scrape_interval): Scrape interval hint set as a pod annotation on the DCGM exporter.
- [additional_node_groups_config](/configuration/rack-parameters/gcp/additional_node_groups_config): Create GPU node pools that the exporter schedules onto.

## Version Requirements
This feature requires at least Convox rack version `3.25.3`.
