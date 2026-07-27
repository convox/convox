---
title: "gpu_observability_enable"
description: "The gpu_observability_enable GCP rack parameter installs the NVIDIA DCGM exporter to export GPU metrics like utilization, memory, and temperature on GKE GPU nodes."
slug: gpu_observability_enable
url: /configuration/rack-parameters/gcp/gpu_observability_enable
---

# gpu_observability_enable

## Description
Enable GPU observability infrastructure (DCGM exporter) on this rack. DCGM is a per-node workload that exports GPU metrics (utilization, memory, temperature) on port 9400.

On GKE the NVIDIA device plugin and drivers are managed by the platform, so `nvidia_device_plugin_enable` is not required and is not a GCP rack parameter. Enabling observability is a single switch.

Setting this parameter to `true` installs the DCGM exporter in the `kube-system` namespace, exposes it on port 9400 through a `dcgm-exporter` ClusterIP Service, and creates a `convox-dcgm-metrics` counter ConfigMap plus six Grafana dashboard ConfigMaps in `kube-system` labeled `grafana_dashboard=1` for sidecar discovery.

The exporter also carries `prometheus.io/*` pod annotations. The chart is installed with `serviceMonitor.enabled=false`, so a Prometheus Operator setup needs its own `ServiceMonitor` or annotation-based discovery. Google Managed Prometheus does not read `prometheus.io/*` annotations at all: it collects through a `PodMonitoring` resource that you create yourself, and Convox does not create one. GCP racks do not deploy a Convox-managed Prometheus.

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

Disabling on the current version (above) is always safe. If you plan to downgrade the rack to a version earlier than the one that introduced GPU observability (`3.25.3`), first set `gpu_observability_enable=false` and let that update complete, then downgrade. This removes the DCGM Helm release while the rack still knows how to manage it.

Downgrading while the exporter is still enabled makes the downgrade apply fail with `Error: Provider configuration not present`, naming `helm_release.dcgm_exporter[0] (orphan)` and the `hashicorp/helm` provider. The `helm` provider is new to the GCP cluster module in `3.25.3`, so the older module has no provider left to destroy the release with.

This blocks the downgrade, it does not wedge the Rack. The cluster keeps running its current version and its workloads are untouched. The failed apply does leave the Rack's stored Terraform configuration pointing at the older version with `gpu_observability_enable` already stripped out, so setting the parameter to `false` at that point re-runs the same failing apply. To recover, return the Rack to `3.25.3` with `convox rack update 3.25.3`, then set `gpu_observability_enable=false` and let that apply finish, then retry the downgrade.

## Additional Information
- GKE manages the NVIDIA device plugin and drivers for GPU node pools, so there is no `nvidia_device_plugin_enable` parameter on GCP.
- The DCGM exporter pod schedules only on nodes carrying the `convox.io/gpu-vendor=nvidia` label, which the rack controller applies at runtime when a node's machine type is a GPU family (`g2-`, `a2-`, `a3-`, `a4-`, `a4x-`, `g4-`). If you have no GPU nodes, the exporter is created but no pods are scheduled.
- The dashboard ConfigMaps do nothing on their own. A Grafana you run yourself picks them up only if its dashboard sidecar watches `kube-system` for the `grafana_dashboard=1` label.
- `convox ps` GPU enrichment and the Console GPU dashboards are AWS-only for now. GCP racks do not deploy a Convox-managed Prometheus and do not wire `prometheus_url`; point your own Prometheus and Grafana at the exporter instead.

> **N1 machine types are not auto-detected.** GPU nodes are identified by machine family (`g2-`, `a2-`, `a3-`, `a4-`, `a4x-`, `g4-`). N1 machines with attached GPUs cannot be detected by machine type alone and will not receive the `convox.io/gpu-vendor=nvidia` label, so the DCGM exporter will not schedule onto them. Prefer the dedicated GPU families, or label such nodes manually.

## Related Parameters
- [gpu_observability_chart_version](/configuration/rack-parameters/gcp/gpu_observability_chart_version): Pin the DCGM exporter chart version.
- [dcgm_scrape_interval](/configuration/rack-parameters/gcp/dcgm_scrape_interval): Scrape interval hint set as a pod annotation on the DCGM exporter.
- [additional_node_groups_config](/configuration/rack-parameters/gcp/additional_node_groups_config): Create GPU node pools that the exporter schedules onto.

## Version Requirements
This feature requires at least Convox rack version `3.25.3`.
