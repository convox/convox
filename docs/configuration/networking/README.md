---
title: "Networking"
slug: networking
url: /configuration/networking
---
# Networking

Convox provides several networking features to control how traffic reaches your services and how services communicate with each other.

## Load Balancers

Every service with a `port` definition automatically gets a load balancer. You can configure SSL termination, end-to-end encryption, custom ports, and dedicated balancers for non-HTTP protocols.

See [Load Balancers](/configuration/load-balancers) for details.

## Service Discovery

Services within the same rack can communicate using internal hostnames. Services with ports are accessible via load balancer hostnames, while internal services communicate directly.

See [Service Discovery](/configuration/service-discovery) for details.

## Health Checks

Health checks verify that your services are running correctly. Convox supports HTTP health checks, liveness probes, startup probes, and gRPC health checks.

See [Health Checks](/configuration/health-checks) for details.

## Rack-to-Rack Communication

For services that need to communicate across racks, you can configure internal routers and private networking between VPC-peered racks.

See [Rack-to-Rack Communication](/configuration/rack-to-rack) for details.
