---
title: "dcgm_scrape_interval"
description: "The dcgm_scrape_interval GCP rack parameter sets the Prometheus scrape interval hint annotated on the DCGM exporter for GPU metrics, defaulting to 15s."
slug: dcgm_scrape_interval
url: /configuration/rack-parameters/gcp/dcgm_scrape_interval
---

# dcgm_scrape_interval

## Description
The `dcgm_scrape_interval` parameter sets the value published as the DCGM exporter's `prometheus.io/scrape-interval` pod annotation. The annotation is a hint. Setting it does not change how often anything scrapes the exporter until your collector is configured to act on it.

| Collector | What the annotation does | What you must do |
|:----------|:-------------------------|:-----------------|
| A Prometheus you run yourself | Reaches the target as a pod annotation meta label | Relabel it into `__scrape_interval__`, or set the interval on the scrape job directly |
| Google Managed Prometheus | Nothing. GMP does not read `prometheus.io/*` annotations | Create a `PodMonitoring` resource targeting the exporter and set the interval on it. Convox does not create one |

Lower values produce more responsive charts at the cost of more Prometheus storage and CPU; higher values reduce overhead at the cost of coarser resolution.

Accepts a duration string, for example `15s`, `30s`, or `2m`.

## Default Value
The default value is `15s`.

## Allowed Range
`15s` to `300s` (5 minutes). Values below `15s` exceed the DCGM exporter's recommended scrape budget; values above `300s` render mostly gaps. Values outside the range, or values that are not valid durations, are rejected.

## Use Cases
- **Cost-sensitive Prometheus storage**: Bump from `15s` to `30s` or `60s` to reduce sample volume on metered Prometheus tiers.
- **Tighter resolution for active debugging**: Keep `15s` (the default) when chasing a transient GPU spike.

## Setting Parameters
To bump scrape interval to 30 seconds:
```bash
$ convox rack params set dcgm_scrape_interval=30s -r rackName
Updating parameters... OK
```

To clear the override (falls back to the rack default `15s`):
```bash
$ convox rack params set dcgm_scrape_interval= -r rackName
Updating parameters... OK
```

## Operational Notes
- If you raise the value to `60s` and your Prometheus keeps scraping at its job default, the collector is not consuming the annotation. Apply the relabel or `PodMonitoring` change from the table above.
- The Prometheus scrape interval and the DCGM exporter's internal collection interval are independent. DCGM collects continuously; this parameter controls only the annotation value.
- Changing this parameter rewrites a pod annotation, which rolls the DCGM exporter pods.
- `dcgm_scrape_interval` is a no-op when `gpu_observability_enable=false`.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/gcp/gpu_observability_enable): The enable switch for the DCGM exporter chart.
- [gpu_observability_chart_version](/configuration/rack-parameters/gcp/gpu_observability_chart_version): Pin the DCGM exporter chart version.

## Version Requirements
This parameter requires at least Convox rack version `3.25.3`.
