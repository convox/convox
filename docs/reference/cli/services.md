---
title: "services"
slug: services
url: /reference/cli/services
---
# services

The `convox services` command lists the services defined for an app along with their domains and port mappings. Use `convox services restart` to restart a specific service without affecting the others.

## services

List services for an app

### Usage
```bash
    convox services
```
### Examples
```bash
    $ convox services
    SERVICE  DOMAIN                                PORTS
    web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:3000
```
## services restart

Restart a service

### Usage
```bash
    convox services restart <service>
```
### Examples
```bash
    $ convox services restart web
    Restarting web... OK
```

## See Also

- [Service](/reference/primitives/app/service) for service configuration
- [Load Balancers](/configuration/load-balancers) for load balancer setup