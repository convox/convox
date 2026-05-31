---
title: "gpu_metrics_max_pods"
slug: gpu_metrics_max_pods
url: /configuration/rack-parameters/aws/gpu_metrics_max_pods
---

# gpu_metrics_max_pods

## Description
The `gpu_metrics_max_pods` parameter caps the number of services included in a single GPU metrics request, bounding fan-out from a busy app's per-service GPU dashboard. (The parameter name is historical; the limit applies to the count of services in the request, since each service in turn fans out to its own pod set.) The Console issues one request per chart load and per dropdown change; without a cap, an app with many services can flood the rack with simultaneous Prometheus queries.

A request whose service list exceeds the cap is rejected with HTTP 400.

## Default Value
The default value is `100`.

## Allowed Range
`1` to `500`. The upper bound prevents an arbitrarily high value from removing the protection this cap provides. Values outside the `1` to `500` range are rejected.

## Use Cases
- **Apps with very large GPU pod counts**: Operators running 200+ GPU pods in a single service can bump to `200` or `300` to surface every pod in the dashboard, accepting the higher Prometheus load.
- **Tight DoS protection on shared racks**: Operators running multi-tenant racks where one app's busy chart should not impact others can drop the value to `50` or lower.

## Setting Parameters
To raise the pod cap to 200:
```bash
$ convox rack params set gpu_metrics_max_pods=200 -r rackName
Setting parameters... OK
```

To revert to the default:
```bash
$ convox rack params set gpu_metrics_max_pods=100 -r rackName
Setting parameters... OK
```

To clear the override (falls back to the default `100`):
```bash
$ convox rack params set gpu_metrics_max_pods= -r rackName
Setting parameters... OK
```

## Operational Notes
- The cap is request-scoped: it limits the count of services the request asks for in a single fetch, not the total pod count those services back onto.
- A request whose service set would exceed the cap returns HTTP 400; the Console surfaces this as a banner suggesting a smaller service selection or a higher cap.
- The cap protects only the chart endpoint; per-pod summary cards in the service detail page are not bounded by this parameter.

## Related Parameters
- [gpu_metrics_max_concurrent](/configuration/rack-parameters/aws/gpu_metrics_max_concurrent): Companion cap on simultaneous Prometheus queries for GPU metrics.
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): The enable switch for GPU observability. `gpu_metrics_max_pods` has no effect when `gpu_observability_enable=false`.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
