---
title: "prometheus_gpu_metrics_retention"
slug: prometheus_gpu_metrics_retention
url: /configuration/rack-parameters/aws/prometheus_gpu_metrics_retention
---

# prometheus_gpu_metrics_retention

## Description
The `prometheus_gpu_metrics_retention` parameter sets the retention window for the free-path GPU-metrics Prometheus installed when [`gpu_observability_enable=true`](/configuration/rack-parameters/aws/gpu_observability_enable) and the Convox metered metrics offering is NOT enabled. The free Prometheus is scoped to scraping the NVIDIA DCGM exporter only and serves the rack-API's GPU metric enrichment for `convox ps` and Console GPU dashboards.

Accepts Prometheus duration syntax — for example `24h`, `7d`, `2h30m`, or `54s321ms`.

## Default Value
The default value is `24h`.

## Use Cases
- **Longer-window debugging**: Bump retention from `24h` to `48h` or `72h` when investigating a bug that surfaces over a multi-day window.
- **Tighter retention to reduce memory**: For racks with many GPU pods, drop retention to `12h` to halve the memory footprint of the free Prometheus.

## Setting Parameters
To change retention:
```bash
$ convox rack params set prometheus_gpu_metrics_retention=48h -r rackName
Setting parameters... OK
```

To revert to the default:
```bash
$ convox rack params set prometheus_gpu_metrics_retention=24h -r rackName
Setting parameters... OK
```

## Operational Notes
- Storage is `emptyDir` (pod-local, ephemeral). Pod restart drops data.
- Sizing rule-of-thumb: budget 1Gi memory per 50 concurrent GPU pods at default 24h retention. Doubling retention to 48h doubles memory needs.
- Default 24h is sufficient for "what was happening yesterday" debugging. For longer-window analysis, enable Convox metered metrics (paid tier) which provides remote_write to managed long-term storage.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): The enable switch that controls whether the free chart is installed.
- [prometheus_gpu_metrics_chart_version](/configuration/rack-parameters/aws/prometheus_gpu_metrics_chart_version): Pin the chart version for the free-path Prometheus.
- `monitoring_metrics_provisioned`: Internal flag set by the Convox metered metrics offering. When `true`, the free chart is suppressed. Not user-settable.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
