---
title: "dcgm_scrape_interval"
description: "The dcgm_scrape_interval AWS rack parameter controls how often the rack-managed Prometheus job scrapes the DCGM exporter for GPU metrics, defaulting to 15s."
slug: dcgm_scrape_interval
url: /configuration/rack-parameters/aws/dcgm_scrape_interval
---

# dcgm_scrape_interval

## Description
The `dcgm_scrape_interval` parameter controls how often the rack-managed Prometheus job scrapes the DCGM exporter for GPU metrics. Lower values produce a more responsive Console GPU dashboard at the cost of more Prometheus storage and CPU; higher values reduce overhead at the cost of coarser chart resolution.

The interval applies to both metrics paths:
- The free in-cluster Prometheus deployment (Console-managed) uses it as the scrape interval for the DCGM exporter job.
- If you run your own Prometheus that discovers targets via `prometheus.io/*` pod annotations, it picks up the same interval from the DCGM exporter's `prometheus.io/scrape-interval` annotation.

Accepts a duration string, for example `15s`, `30s`, or `2m`.

## Default Value
The default value is `15s`.

## Allowed Range
`15s` to `300s` (5 minutes). Values below `15s` exceed the DCGM exporter's recommended scrape budget; values above `300s` are sparse enough that the chart renders mostly gaps. Values outside the `15s` to `300s` range, or values that are not valid durations, are rejected.

## Use Cases
- **Cost-sensitive Prometheus storage**: Bump from `15s` to `30s` or `60s` to reduce sample volume. Operators on metered Prometheus tiers (Datadog, Grafana Cloud, AMP) save substantially on this metric class.
- **Tighter resolution for active debugging**: Drop to `15s` (the default) when chasing a transient GPU spike that gets averaged out at coarser intervals.

## Setting Parameters
To bump scrape interval to 30 seconds:
```bash
$ convox rack params set dcgm_scrape_interval=30s -r rackName
Setting parameters... OK
```

To revert to the default:
```bash
$ convox rack params set dcgm_scrape_interval=15s -r rackName
Setting parameters... OK
```

To clear the override (falls back to the rack default `15s`):
```bash
$ convox rack params set dcgm_scrape_interval= -r rackName
Setting parameters... OK
```

## Operational Notes
- The Prometheus scrape interval and the DCGM exporter's internal collection interval are independent. DCGM internally collects continuously; this parameter controls only the Prometheus pull cadence.
- Changing this value does not require a DCGM exporter restart. The next Console reconciliation cycle propagates the new interval to the Prometheus scrape config.
- Operators running their own self-installed Prometheus that consumes the `prometheus.io/scrape-interval` annotation see the change after the helm release reconciles.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): The enable switch for the DCGM exporter chart. `dcgm_scrape_interval` is a no-op when `gpu_observability_enable=false`.
- [prometheus_gpu_metrics_retention](/configuration/rack-parameters/aws/prometheus_gpu_metrics_retention): Retention window for the same Prometheus deployment. Pair coarser scrape intervals with longer retention to keep total storage roughly constant.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
