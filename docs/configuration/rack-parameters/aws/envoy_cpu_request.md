---
title: "envoy_cpu_request"
description: "The envoy_cpu_request AWS rack parameter sets the CPU request for the Envoy data-plane pods when router_type=contour, defaulting to 100m."
slug: envoy_cpu_request
url: /configuration/rack-parameters/aws/envoy_cpu_request
---

# envoy_cpu_request

## Description

The `envoy_cpu_request` parameter sets the CPU request for the Envoy data-plane Pods on AWS Racks. Envoy is the data plane that proxies request traffic for your Apps, so its CPU request governs how much guaranteed CPU each Envoy Pod is scheduled with.

This parameter applies only when `router_type=contour`. Under Contour, Envoy terminates and forwards external and internal request traffic. Increasing the request gives the data plane more guaranteed CPU for CPU-bound, high-throughput Racks where Envoy is the bottleneck. On Racks running the default nginx ingress (`router_type=nginx`), this parameter has no effect.

## Default Value

The default value is `100m` (100 millicores). This default preserves existing behavior, so Racks that do not set the parameter run Envoy with the standard CPU request.

## Use Cases

- Raise the request on CPU-bound, high-throughput Racks where Envoy needs more guaranteed CPU to proxy traffic without contention.
- Leave it at the default on Racks with light or moderate request volume, where `100m` is sufficient.

## Setting the Parameter

```bash
$ convox rack params set envoy_cpu_request=200m -r rackName
```
Setting parameters... OK

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## Additional Information

The value uses Kubernetes CPU resource units, where `1000m` equals one full CPU core. Express fractional cores in millicores (for example `100m`, `250m`, `500m`) or whole cores as a plain number (for example `1`).

This parameter is clearable. Clearing it returns Envoy to the default request of `100m`:

```bash
$ convox rack params set envoy_cpu_request= -r rackName
```
Setting parameters... OK

The parameter is AWS only and takes effect only when `router_type=contour`. Setting it on an nginx Rack stores the value but changes nothing until the Rack switches to Contour. Changing this request reschedules the Envoy Pods, which is handled as a rolling update of the data plane.

## See Also

- [Ingress Router](/configuration/ingress-router)
- [router_type](/configuration/rack-parameters/aws/router_type)
- [envoy_memory_request](/configuration/rack-parameters/aws/envoy_memory_request)
- [contour_cpu_request](/configuration/rack-parameters/aws/contour_cpu_request)
