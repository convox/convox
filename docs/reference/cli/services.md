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

When the app's budget cap has been breached (3.24.6+), `convox services`
adds a `BUDGET` column showing the per-service sub-state (`armed-Nm`,
`at-cap-keda`, `at-cap-auto`, `at-cap`) — see [`convox ps`](/reference/cli/ps)
for the full value reference and [Budget Caps](/management/budget-caps)
for recovery flows.

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

## services update

Update a service in place. Mirrors the effect of editing `scale.*` fields in
`convox.yml` and redeploying, without a new release. Accepts the same flags as
`convox scale` (`--count`, `--cpu`, `--memory`, `--gpu`, `--gpu-vendor`).

### Usage
```bash
    convox services update <service> [--count N] [--cpu M] [--memory M] [--gpu N] [--gpu-vendor VENDOR]
```
### Examples
```bash
    $ convox services update web --gpu 1 --gpu-vendor nvidia
    Updating web... OK

    $ convox services update web --count 3 --memory 2048
    Updating web... OK
```

## See Also

- [Service](/reference/primitives/app/service) for service configuration
- [Load Balancers](/configuration/load-balancers) for load balancer setup