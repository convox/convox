---
title: "gpu_metrics_max_concurrent"
description: "The gpu_metrics_max_concurrent AWS rack parameter caps the number of simultaneous Prometheus queries the rack issues for GPU metrics, defaulting to 10."
slug: gpu_metrics_max_concurrent
url: /configuration/rack-parameters/aws/gpu_metrics_max_concurrent
---

# gpu_metrics_max_concurrent

## Description
The `gpu_metrics_max_concurrent` parameter caps the number of simultaneous Prometheus queries the rack will issue for GPU metrics across all in-flight requests. When the cap is reached, additional requests fail immediately with HTTP 503; the Console surfaces this as a "Server is busy, please retry" banner.

The cap is immediate, not a queue: a 503 is returned right away rather than the request waiting. The Console retries on user action (refresh, dropdown change), not automatically.

## Default Value
The default value is `10`.

## Allowed Range
`1` to `50`. The upper bound prevents a single operator from saturating Prometheus on shared racks. Values outside the `1` to `50` range are rejected.

## Use Cases
- **High-fanout apps with many concurrent dashboards**: Operators with several team members watching different per-service GPU charts simultaneously may need to bump from `10` to `20` to avoid 503s.
- **Cost-sensitive Prometheus**: Drop to `5` to push back on chart-load fan-out when running on metered Prometheus.

## Setting Parameters
To raise concurrency to 20:
```bash
$ convox rack params set gpu_metrics_max_concurrent=20 -r rackName
Setting parameters... OK
```

To revert to the default:
```bash
$ convox rack params set gpu_metrics_max_concurrent=10 -r rackName
Setting parameters... OK
```

To clear the override (falls back to the default `10`):
```bash
$ convox rack params set gpu_metrics_max_concurrent= -r rackName
Setting parameters... OK
```

## Operational Notes
- The cap is rack-wide, not per-app. Two apps each loading a GPU dashboard simultaneously share the budget.
- A 503 is a soft signal. The Console surfaces it as a transient banner. Persistent 503s usually indicate the cap is too low for the operator's chart fan-out, not that Prometheus itself is unhealthy.
- The cap does NOT protect Prometheus from runaway query latency. If a single query takes 30s, that slot is unavailable for new requests for the whole window. Pair this parameter with appropriate Prometheus query timeouts.

## Related Parameters
- [gpu_metrics_max_pods](/configuration/rack-parameters/aws/gpu_metrics_max_pods): Companion cap on the number of services included per request (parameter name is historical).
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): The enable switch for GPU observability. `gpu_metrics_max_concurrent` has no effect when `gpu_observability_enable=false`.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
