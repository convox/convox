---
title: "envoy_memory_request"
slug: envoy_memory_request
url: /configuration/rack-parameters/aws/envoy_memory_request
---

# envoy_memory_request

## Description

The `envoy_memory_request` parameter sets the memory request for the Envoy data-plane pods. These pods carry every request that enters the Rack through the ingress router, so the memory they reserve affects how much in-flight traffic and response data they can buffer.

This parameter applies only when `router_type` is set to `contour`. On Racks running the default nginx router it has no effect. It is available on AWS Racks.

## Default Value

The default value is `256Mi`. This default preserves existing behavior, so Racks that do not set it run the Envoy data plane with the same memory request as before.

## Use Cases

- Raise the value for high-traffic Racks where the Envoy pods handle a large volume of concurrent connections.
- Raise the value for Racks serving large responses, so the data plane has headroom to buffer response bodies.

## Setting the Parameter

```bash
$ convox rack params set envoy_memory_request=512Mi -r rackName
```
Setting parameters... OK

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## Additional Information

The value uses Kubernetes resource quantity notation, such as `256Mi` or `1Gi`. The request is the amount of memory reserved for each Envoy pod; the scheduler uses it to place the pods on nodes with sufficient capacity.

This parameter is clearable. Clearing it returns the Envoy data plane to the `256Mi` default:

```bash
$ convox rack params set envoy_memory_request= -r rackName
```
Setting parameters... OK

Because this parameter only takes effect when `router_type=contour`, setting it on a Rack still running nginx records the value but does not change any running pods until you switch the Rack to Contour.

## See Also

- [Ingress Router](/configuration/ingress-router)
- [router_type](/configuration/rack-parameters/aws/router_type)
- [envoy_cpu_request](/configuration/rack-parameters/aws/envoy_cpu_request)
- [contour_memory_request](/configuration/rack-parameters/aws/contour_memory_request)
