---
title: "high_availability"
description: "The high_availability Azure rack parameter runs the router and API with multiple replicas when true, or single replicas when false, defaulting to true."
slug: high_availability
url: /configuration/rack-parameters/azure/high_availability
---

# high_availability

## Description
The `high_availability` parameter determines whether to create a high availability (HA) cluster. When enabled, the router (nginx) and API components run with multiple replicas for redundancy. When disabled, they run with a single replica, reducing resource consumption.

## Default Value
The default value for `high_availability` is `true`.

## Use Cases
- **Cost Optimization**: Reduce infrastructure costs by running fewer replicas of system components.
- **Development Environments**: Use single-replica mode for non-production clusters where uptime is not critical.
- **Production Resilience**: Keep enabled in production for increased fault tolerance.

## Setting Parameters
The `high_availability` parameter can only be set when the Rack is installed:
```bash
$ convox rack install azure <name> high_availability=false
```
It cannot be changed afterwards. `convox rack params set` rejects it with `param 'high_availability' can only be set during rack installation`.

## Additional Information
When `high_availability` is `true`, the nginx ingress controller runs with 2-10 replicas (autoscaled) and the API runs with 2 replicas. When `false`, both run with a single replica.
