---
title: "prometheus_gpu_metrics_retention"
description: "The prometheus_gpu_metrics_retention AWS rack parameter sets the retention window for the free-plan Prometheus deployed via the Convox Console, defaulting to 24h."
slug: prometheus_gpu_metrics_retention
url: /configuration/rack-parameters/aws/prometheus_gpu_metrics_retention
---

# prometheus_gpu_metrics_retention

## Description
The `prometheus_gpu_metrics_retention` parameter sets the retention window for the free-plan Prometheus chart deployed via the Convox Console. Applies on next Disable→Enable cycle from the Convox Console. The free Prometheus is scoped to scraping the NVIDIA DCGM exporter only and serves the rack-API's GPU metric enrichment for `convox ps` and Console GPU dashboards.

Accepts Prometheus duration syntax, for example `24h`, `7d`, `2h30m`, or `54s321ms`. Storage is `emptyDir`; longer retention requires more memory.

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

To clear the override, equivalent to reverting to default since the Console worker reads the empty value as "use the default":
```bash
$ convox rack params set prometheus_gpu_metrics_retention= -r rackName
Setting parameters... OK
```

## Operational Notes
- Storage is `emptyDir` (pod-local, ephemeral). Pod restart drops data.
- Sizing rule-of-thumb: budget 1Gi memory per 50 concurrent GPU pods at default 24h retention. Doubling retention to 48h doubles memory needs.
- Default 24h is sufficient for "what was happening yesterday" debugging. For longer-window analysis, enable Convox metered metrics (paid tier) which provides remote_write to managed long-term storage.
- Clearing this parameter resets retention to the default `24h`. Existing samples are retained until the new retention window expires them; the chart is upgraded in place (no data loss from the clear itself, only from the natural retention rolloff).
- Increasing retention does NOT retroactively recover data older than the previous window. The chart's TSDB only holds samples that fell within whatever retention was in effect when those samples were written; a stretch from `24h` to `7d` widens the window going forward, it does not undo prior compaction.
- Param updates that land while a rack-level update is in flight (Terraform state lock held) return the canonical `state lock` error. Re-issue the change after the in-flight rack update finishes; the Console reconciler converges to the new retention on the next tick.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Enables the DCGM exporter on this rack. The Console-deployed Prometheus chart scrapes DCGM when monitoring is enabled in the Console.
- [prometheus_gpu_metrics_chart_version](/configuration/rack-parameters/aws/prometheus_gpu_metrics_chart_version): Pin the chart version for the free-plan Prometheus chart deployed via the Convox Console.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
