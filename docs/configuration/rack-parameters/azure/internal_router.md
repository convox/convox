---
title: "internal_router"
slug: internal_router
url: /configuration/rack-parameters/azure/internal_router
---

# internal_router

## Description
The `internal_router` parameter enables an additional internal load balancer for routing traffic within the virtual network. When enabled, a second Azure Load Balancer is created with the `service.beta.kubernetes.io/azure-load-balancer-internal` annotation, allowing services to be accessed from within the VNet without exposing them to the public internet.

## Default Value
The default value for `internal_router` is `false`.

## Use Cases
- **Private Services**: Expose services only within the Azure virtual network.
- **Microservice Communication**: Route internal traffic between services without going through the public internet.
- **Security**: Restrict access to backend services to internal network consumers only.

## Setting Parameters
To set the `internal_router` parameter, use the following command:
```bash
$ convox rack params set internal_router=true -r rackName
Setting parameters... OK
```

## Additional Information
When enabled, a separate `router-internal` Kubernetes service is created with an internal Azure Load Balancer. The internal router uses the same nginx ingress controller with a dedicated selector for internal traffic routing.
