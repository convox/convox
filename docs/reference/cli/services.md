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

## services triggers enable

Enable a Console-driven autoscaler for a service. Requires rack version
3.24.6 or later. See [Autoscale Triggers Override](/console/autoscale-triggers)
for the full surface (Console + CLI parity).

### Usage
```bash
    convox services triggers enable <service> \
        --min <int> --max <int> \
        [--cpu <1-100>] [--memory <1-100>] \
        [--gpu <1-100>] [--queue <int>]
```

At least one of `--cpu`, `--memory`, `--gpu`, `--queue` is required.
`--gpu` and `--queue` require KEDA on the rack (`keda_enable=true`).
`--gpu` additionally requires the service to declare `scale.gpu.count >= 1`
in `convox.yml`.

CPU- or memory-only overrides materialize a native Kubernetes HPA and
require `--min` >= 1; the Kubernetes `HPAScaleToZero` feature gate is
alpha and not enabled on managed clusters. For scale-to-zero behavior,
include a KEDA-eligible trigger (`--gpu` or `--queue`) — the KEDA
`ScaledObject` path supports `--min 0` natively. Mixed trigger sets
that include any of `--gpu` / `--queue` (e.g. `--cpu 70 --gpu 75`)
dispatch through the KEDA `ScaledObject` path and therefore also
accept `--min 0`.

### Examples
```bash
    $ convox services triggers enable web --min 1 --max 5 --cpu 70
    Enabling triggers override on web... OK

    $ convox services triggers enable worker --min 0 --max 10 --gpu 75 --queue 100
    Enabling triggers override on worker... OK
```

## services triggers disable

Remove the Console-driven autoscaler. The next deploy re-materializes
the manifest's autoscale config (if any).

### Usage
```bash
    convox services triggers disable <service>
```

### Examples
```bash
    $ convox services triggers disable web
    Disabling triggers override on web... OK
```

## services triggers threshold-set

Update a single trigger's threshold on a service that already has an
override active. `--type` accepts `cpu`, `memory`, `gpu`, or `queue`.

### Usage
```bash
    convox services triggers threshold-set <service> \
        --type <cpu|memory|gpu|queue> --threshold <number>
```

### Examples
```bash
    $ convox services triggers threshold-set web --type cpu --threshold 80
    Setting web cpu threshold to 80... OK
```

## See Also

- [Service](/reference/primitives/app/service) for service configuration
- [Load Balancers](/configuration/load-balancers) for load balancer setup
- [Autoscale Triggers Override](/console/autoscale-triggers) for the
  Console UI behind these CLI subcommands