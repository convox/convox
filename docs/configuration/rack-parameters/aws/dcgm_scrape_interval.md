---
title: "dcgm_scrape_interval"
slug: dcgm_scrape_interval
url: /configuration/rack-parameters/aws/dcgm_scrape_interval
---

# dcgm_scrape_interval

## Description
The `dcgm_scrape_interval` parameter controls how often the rack-managed Prometheus job scrapes the DCGM exporter for GPU metrics. Lower values produce a more responsive Console GPU dashboard at the cost of more Prometheus storage and CPU; higher values reduce overhead at the cost of coarser chart resolution.

The value flows two ways:
- The free in-cluster Prometheus path (Convox-Console-managed) reads the value via `pkg/structs/dcgm-scrape.yaml` and writes it into the scrape job's `scrape_interval` field.
- Operators running their own Prometheus that discovers via `prometheus.io/*` pod annotations pick up the same interval from the DaemonSet's `prometheus.io/scrape-interval` annotation set in `terraform/cluster/aws/dcgm.tf`.

Accepts a Go-style duration string — for example `15s`, `30s`, or `2m`.

## Default Value
The default value is `15s`.

## Allowed Range
`15s` to `300s` (5 minutes). Values below `15s` exceed the DCGM exporter's recommended scrape budget; values above `300s` are sparse enough that the chart renders mostly gaps. The validator at `pkg/cli/rack.go` rejects out-of-range values.

## Use Cases
- **Cost-sensitive Prometheus storage**: Bump from `15s` to `30s` or `60s` to reduce sample volume — operators on metered Prometheus tiers (Datadog, Grafana Cloud, AMP) save substantially on this metric class.
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
