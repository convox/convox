---
title: "contour_cpu_request"
description: "The contour_cpu_request AWS rack parameter sets the CPU request for the Contour control-plane pods when router_type=contour, defaulting to 100m."
slug: contour_cpu_request
url: /configuration/rack-parameters/aws/contour_cpu_request
---

# contour_cpu_request

## Description

The `contour_cpu_request` parameter sets the CPU request for the Contour control-plane Pods on a Rack. It applies only when `router_type=contour`. The value is a standard Kubernetes CPU quantity (for example `100m` or `250m`).

This parameter has no effect on nginx Racks. On a Rack with `router_type=nginx`, the value is ignored regardless of what it is set to.

## Default Value

The default value is `100m`. This default preserves existing behavior: a Rack that has not set `contour_cpu_request` schedules the Contour control plane with a `100m` CPU request, the same value used before this parameter was available.

## Use Cases

- Raise the request when the Contour control plane is CPU constrained, for example on a Rack with many routes where reconciliation is slow.
- Lower the request on small or low-traffic Racks to free CPU for other workloads on the same nodes.

## Setting the Parameter

```bash
$ convox rack params set contour_cpu_request=250m -r rackName
```
Setting parameters... OK

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## Additional Information

This parameter is available on AWS Racks only and takes effect when `router_type=contour`.

`contour_cpu_request` is clearable. Clearing it (setting it to an empty value) restores the default of `100m`. Setting it to a new value reschedules the Contour control-plane Pods with the updated CPU request.

Because changing this value reschedules the control-plane Pods, apply it during a window where a brief Contour control-plane restart is acceptable.

## See Also

- [Ingress Router](/configuration/ingress-router)
- [router_type](/configuration/rack-parameters/aws/router_type)
- [contour_memory_request](/configuration/rack-parameters/aws/contour_memory_request)
- [envoy_cpu_request](/configuration/rack-parameters/aws/envoy_cpu_request)
