---
title: "gpu_metrics_max_pods"
slug: gpu_metrics_max_pods
url: /configuration/rack-parameters/aws/gpu_metrics_max_pods
---

# gpu_metrics_max_pods

## Description
The `gpu_metrics_max_pods` parameter caps the number of pods returned by a single rack-side GPU metrics request, bounding fan-out from a busy app's per-service GPU dashboard. The Console issues one request per chart load and per dropdown change; without a cap, an app with hundreds of GPU pods can flood the rack with simultaneous Prometheus queries.

The cap is enforced at the request boundary in the rack handler. Pods beyond the cap are not queried for that response.

## Default Value
The default value is `100`.

## Allowed Range
`1` to `500`. The upper bound prevents an operator from defeating the DoS bound by setting an arbitrarily high value. The validator at `pkg/cli/rack.go` rejects values above 500 or non-positive.

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

To clear the override (falls back to the handler default `100`):
```bash
$ convox rack params set gpu_metrics_max_pods= -r rackName
Setting parameters... OK
```

## Operational Notes
- The cap is request-scoped, not service-scoped. A request asking for 5 services with 50 pods each is allowed at the default 100 cap only if the total queried pod count fits.
- The handler returns a 400 if the requested service set would exceed the cap; the Console surfaces this as a banner suggesting a smaller service selection or a higher cap.
- The cap protects only the chart endpoint; per-pod summary cards in the service detail page are not bounded by this parameter.

## Related Parameters
- [gpu_metrics_max_concurrent](/configuration/rack-parameters/aws/gpu_metrics_max_concurrent): Companion cap on simultaneous Prometheus QueryRange invocations from the GPU metrics handler.
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): The enable switch for the DCGM exporter chart. `gpu_metrics_max_pods` is a no-op when `gpu_observability_enable=false`.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
