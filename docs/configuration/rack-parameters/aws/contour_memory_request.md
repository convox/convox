---
title: "contour_memory_request"
description: "The contour_memory_request AWS rack parameter sets the memory request for the Contour control-plane pods when router_type=contour, defaulting to 128Mi."
slug: contour_memory_request
url: /configuration/rack-parameters/aws/contour_memory_request
---

# contour_memory_request

Memory request for the Contour control-plane pods.

## Description

The `contour_memory_request` parameter sets the Kubernetes memory request applied to the Contour control-plane pods on a Rack. It takes effect when `router_type=contour`. The Contour control plane watches your routing configuration and translates it into Envoy configuration, so the amount of memory it needs grows with the number of routes the Rack serves.

On a Rack with many routes, the control plane holds a larger configuration in memory. Raising `contour_memory_request` reserves enough memory for that configuration and keeps the control plane from running low under load.

This parameter has no effect on Racks running `router_type=nginx`. It applies only to AWS Racks using Contour.

## Default Value

The default value is `128Mi`. This default preserves existing behavior, so Racks that do not set the parameter run the Contour control plane with the standard memory request.

## Use Cases

- Raise the value on Racks with many routes, where the Contour control plane holds a larger configuration and can otherwise run low on memory.
- Keep the default on Racks with a small number of routes, where `128Mi` is sufficient for the control plane.

## Setting the Parameter

```bash
$ convox rack params set contour_memory_request=256Mi -r rackName
```
Setting parameters... OK

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## Additional Information

This parameter is clearable. Clearing it returns the control plane to the default request of `128Mi`:

```bash
$ convox rack params set contour_memory_request= -r rackName
```
Setting parameters... OK

The value is a Kubernetes memory quantity (for example `128Mi`, `256Mi`, or `512Mi`). It sets a request, not a limit, so it reserves scheduling capacity rather than capping usage. The parameter applies only when `router_type=contour`; on `router_type=nginx` Racks it has no effect regardless of its value. It is available on AWS Racks only.

## See Also

- [Ingress Router](/configuration/ingress-router)
- [router_type](/configuration/rack-parameters/aws/router_type)
- [contour_cpu_request](/configuration/rack-parameters/aws/contour_cpu_request)
- [envoy_memory_request](/configuration/rack-parameters/aws/envoy_memory_request)
