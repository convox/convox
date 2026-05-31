---
title: "Migrating from Convox v2 to v3"
slug: v2-to-v3
url: /migration/v2-to-v3
---
# Migrating from Convox v2 to v3

This guide is for teams running apps on a Convox v2 (ECS/CloudFormation) rack who want to move them to a v3 (Kubernetes) rack. The practical reason to migrate is that v3 is the actively developed engine: it runs on EKS, GKE, AKS, DigitalOcean, and bare metal, and it carries the newer manifest features (KEDA autoscaling, VPA, GPU scheduling, config mounts, per-service security contexts) that are not available on v2. The good news is that the `convox.yml` manifest is mostly the same between the two engines. Most apps move over by copying the manifest and applying a small number of targeted edits, which this page enumerates field by field.

This is a `convox.yml` diff guide. It does not cover moving data between databases or repointing DNS in detail beyond the cutover section; it focuses on what changes in your manifest.

## Prerequisites

- A running Convox v3 rack and the `convox` CLI. See [Production Rack](/installation/production-rack) to set up a v3 rack and [CLI](/reference/cli) to install the client.
- A `Dockerfile` for each service. v3 builds images from a `Dockerfile`. If any of your v2 services relied on a buildpack-style build (no `Dockerfile` in the source), you will need to add one before deploying on v3. See [Dockerfile](/configuration/dockerfile).
- Your existing v2 `convox.yml` and the list of environment variables for the app (`convox env -a <app>` on the v2 rack).

## Concept mapping

| Convox v2 concept | Convox v3 equivalent |
|-------------------|----------------------|
| ECS service | Kubernetes Deployment (still a `services:` entry in `convox.yml`) |
| ECS task / one-off process | `convox run` process (same command) |
| CloudFormation stack per app | Kubernetes namespace per app (managed by the rack) |
| `links:` between services | [Service discovery](/configuration/service-discovery) by service name, plus shared `resources:` |
| Service-level `nlb:` block | [Custom Balancers](/reference/primitives/app/balancer) (`balancers:`) and service `ports:` |
| `scale.targets` (HPA) | `scale.targets` (still supported) or the new `scale.autoscale` / KEDA blocks |
| ELB/ALB managed by the rack | Rack router (NGINX/Envoy ingress), still configured via `port:` and `domain:` |
| Rack add-on resources (`type: postgres`, etc.) | Same `resources:` block, now containerized in-cluster, AWS RDS/ElastiCache, or [Convox Cloud databases](/cloud/databases) |

## Is `convox apps export` / `import` a clean path?

Not as a general v2-to-v3 migration. `convox apps export` / `convox apps import` works for some apps, but it is not a reliable engine-to-engine path because the two engines back the app with different infrastructure (ECS vs Kubernetes) and the export bundle does not always translate cleanly. The recommended approach is the one this page describes: take your v2 `convox.yml`, copy it to the v3 app, and apply the targeted field changes below. The manifest is the source of truth and it is mostly portable.

## convox.yml: what is the same

The top-level shape is identical. Both engines parse `environment`, `resources`, `services`, and `timers` the same way, and within a service the following fields are byte-for-byte compatible:

`build` (path or `{path, manifest, args}`), `image`, `command` (as a string), `environment`, `port` (`PORT` or `scheme:PORT`), `domain`, `health` (path or `{path, grace, interval, timeout}`), `internal`, `resources`, `scale.count`, `scale.cpu`, `scale.memory`, `scale.targets` (`cpu`, `memory`, `requests`, `custom`), `singleton`, `sticky`, `privileged`, `test`, `volumes`, `drain`, `deployment` (`minimum` / `maximum`), and `termination.grace`.

The default values for these also match (for example `drain: 30`, `deployment.minimum: 50`, `deployment.maximum: 200`, health `interval: 5`), so a service that only uses the fields above generally needs no edits.

A minimal app that uses only shared fields requires no manifest changes:

```yaml
# Works as-is on both v2 and v3
environment:
  - DATABASE_URL
resources:
  database:
    type: postgres
services:
  web:
    build: .
    command: bin/web
    port: 3000
    health: /check
    resources:
      - database
    scale:
      count: 2
      cpu: 256
      memory: 512
```

## Renamed, moved, or changed-syntax fields

These are the fields whose meaning is the same but whose shape changed. Check each one in your manifest.

### `command` no longer accepts a list

On v2, `command` accepted either a string (wrapped as `sh -c`) or a YAML list. On v3, `command` is a plain string. If you used the list form, convert it to a single string.

Before (v2):
```yaml
services:
  worker:
    command:
      - bin/worker
      - --queue=default
```

After (v3):
```yaml
services:
  worker:
    command: bin/worker --queue=default
```

### Agent ports move to the service level

On v2, an agent declared its ports inside the `agent:` block (`agent: { enabled: true, ports: [...] }`). On v3 the agent block is just an on/off switch; v3 will return an error (`agent ports are now specified at the service level`) if you nest ports under `agent`. Declare ports on the service instead.

Before (v2):
```yaml
services:
  metrics:
    agent:
      enabled: true
      ports:
        - 8125/udp
```

After (v3):
```yaml
services:
  metrics:
    agent: true
    ports:
      - 8125/udp
```

See [Agents](/configuration/agents).

### Timer schedules use 5-field cron

On v2, timer `schedule` expressions could use the 6-field AWS cron syntax (with `?` placeholders). v3 uses standard 5-field cron. v3 normalizes on load (it truncates anything past the first five fields and rewrites `?` to `*`), so most v2 schedules keep working, but you should write 5-field cron going forward.

Before (v2, 6-field AWS style):
```yaml
timers:
  cleanup:
    schedule: "0 3 * * ? *"
    command: bin/cleanup
    service: worker
```

After (v3, 5-field):
```yaml
timers:
  cleanup:
    schedule: "0 3 * * *"
    command: bin/cleanup
    service: worker
```

See [Timer](/reference/primitives/app/timer).

## New in v3

These fields do not exist on v2. They are all optional, so adding them is not required to migrate, but they are the reason most teams move.

Top-level manifest sections new in v3:

| Section | Purpose | Reference |
|---------|---------|-----------|
| `labels` | Labels applied to all services | [convox.yml](/configuration/convox-yml#labels) |
| `configs` | Named config objects mounted into containers as files | [Config Mounts](/configuration/config-mounts) |
| `balancers` | Custom TCP/UDP load balancers for arbitrary ports | [Balancer](/reference/primitives/app/balancer) |
| `budget` | Monthly cost caps and at-cap actions | [Budget Caps](/management/budget-caps) |
| `appSettings` | Per-app platform settings (e.g. CloudWatch retention) | [App Settings](/configuration/app-settings) |

Service-level fields new in v3 include: `annotations` and `ingressAnnotations` (Kubernetes annotations), `labels`, `nodeSelectorLabels` / `nodeAffinityLabels` ([Workload Placement](/configuration/scaling/workload-placement)), `init` (init process, default `true`), `liveness` and `startupProbe` ([Health Checks](/configuration/health-checks)), `lifecycle` (preStop / postStart hooks), `securityContext`, `configMounts`, `volumeOptions` (emptyDir / EFS / Azure Files), `imagePullSecrets` ([Private Registries](/configuration/private-registries)), `tls` (redirect control), `internalRouter`, `timeout`, and `certificate`.

The `scale` block also gains several v3-only sub-blocks:

| Scale field | Purpose | Reference |
|-------------|---------|-----------|
| `scale.min` / `scale.max` | Replica bounds used with autoscaling | [Autoscaling](/configuration/scaling/autoscaling) |
| `scale.autoscale` | Preconfigured KEDA triggers (cpu, memory, gpuUtilization, queueDepth) | [Autoscaling](/configuration/scaling/autoscaling#event-driven-autoscaling-scaleautoscale) |
| `scale.keda` | Raw KEDA ScaleTriggers | [KEDA Autoscaling](/configuration/scaling/keda) |
| `scale.vpa` | Vertical Pod Autoscaler | [VPA](/configuration/scaling/vpa) |
| `scale.gpu` | GPU count and vendor per process | [Autoscaling](/configuration/scaling/autoscaling#gpu-scaling) |
| `scale.limit` | CPU/memory limits (vs. requests) | [Autoscaling](/configuration/scaling/autoscaling) |

You can adopt these incrementally after the app is running on v3.

## Removed or deprecated fields

These v2 fields have no direct v3 equivalent. Remove them and use the replacement noted.

| v2 field | Status on v3 | What to do instead |
|----------|--------------|--------------------|
| service `links:` | Not supported | Internal services are reachable by name; see [Service Discovery](/configuration/service-discovery). Share databases via `resources:`. |
| service `nlb:` block | Not supported | Use [Custom Balancers](/reference/primitives/app/balancer) (`balancers:`) for arbitrary TCP/UDP ports, and service `ports:` for service discovery. |
| service `policies:` | Replaced | On AWS, attach IAM permissions with `accessControl.awsPodIdentity.policyArns` on the service. See the [service reference](/reference/primitives/app/service). Other providers use their own workload-identity mechanism. |
| service `tags:` | Not present in the v3 manifest | Use `labels:` (top-level or per-service) and `annotations:` for metadata. |
| service `internalAndExternal:` | Not present | Use `internal:` plus `internalRouter:` to control internal vs external exposure. See the [service reference](/reference/primitives/app/service). |
| `scale.cooldown` | Not present in the v3 scale block | Cooldown is configured per autoscaler: `scale.autoscale.cooldownPeriod` or the equivalent KEDA field. See [Autoscaling](/configuration/scaling/autoscaling#autoscale-reference). |
| timer `policies:` | Not present on the v3 timer | Timers have no inline policy field. Run the work in a service (which supports `accessControl.awsPodIdentity.policyArns`) or grant permissions at the rack level. |

### Example: replacing `links`

Before (v2):
```yaml
services:
  api:
    build: .
    port: 3000
    links:
      - db
  db:
    image: postgres
```

After (v3). Use a managed `resources:` database and reach internal services by name:
```yaml
resources:
  db:
    type: postgres
services:
  api:
    build: .
    port: 3000
    resources:
      - db
```

## Environment and secrets mapping

Environment handling is identical between engines. The top-level `environment:` block applies to every service, and service-level `environment:` adds to it. Required (no default) and defaulted (`KEY=value`) declarations work the same way.

```yaml
environment:
  - COMPANY=Convox   # default value
  - DATABASE_URL     # required, must be set before deploy
```

There is no separate secrets type in the manifest on either engine; secret values are set as environment variables and stored by the rack. To carry your v2 values over:

1. On the v2 rack: `convox env -a <app>` to list the current values.
2. On the v3 rack: set them with `convox env set KEY=value -a <app>` (batch multiple in one call), then deploy.

For configuration delivered as files rather than env vars, v3 adds `configs:` plus service `configMounts:`. See [Environment Variables](/configuration/environment) and [Config Mounts](/configuration/config-mounts).

## Datastores

The `resources:` block keeps the same syntax. What changes is where the resource runs. On v3 you choose one of three models:

- **Containerized** (`type: postgres`, `mysql`, `mariadb`, `redis`, `memcached`, `postgis`): runs inside the rack cluster. Good for development and staging.
- **AWS Managed** (`type: rds-...`, `type: elasticache-...`): provisions managed RDS/ElastiCache.
- **Convox Cloud databases**: managed databases with simplified configuration. See [Convox Cloud Databases](/cloud/databases).

```yaml
resources:
  database:
    type: postgres
    options:
      storage: 200
services:
  web:
    resources:
      - database
```

See [Resource](/reference/primitives/app/resource) for the full type list and options. Migrating the data itself (dump/restore, replication) is outside the scope of this manifest guide; treat the resource definition and the data copy as two separate steps, and keep the old datastore running until cutover is verified.

## Scheduled jobs, workers, and cron

Both engines express periodic jobs as `timers:` and long-running background work as worker `services:`. These map directly.

A worker service (always running) is just a service with no `port:`:
```yaml
services:
  worker:
    build: .
    command: bin/worker
    resources:
      - queue
```

A scheduled job is a timer that runs a command in the context of a service (remember the 5-field cron change above):
```yaml
timers:
  cleanup:
    schedule: "0 3 * * *"
    command: bin/cleanup
    service: worker
```

See [Timer](/reference/primitives/app/timer). On v3 a timer can also set `concurrency` and `parallelCount`, which are new and optional.

## Deploy and cutover

Once the manifest is updated and your Dockerfiles are in place, deploy to the v3 rack:

1. Point the CLI at the v3 rack (`convox switch <v3-rack>`), or pass `-r <v3-rack>` on each command.
2. Create the app on v3: `convox apps create <app> -r <v3-rack>`.
3. Set environment variables: `convox env set ... -a <app> -r <v3-rack>` (batch all of them in one command).
4. Deploy: `convox deploy -a <app> -r <v3-rack>`. This builds, creates a [Release](/reference/primitives/app/release), and promotes it in one step. See [Deploying Changes](/deployment/deploying-changes).
5. Verify before cutover. Use the rack-assigned hostname (`convox services -a <app>` shows endpoints) and exercise the app on v3 while v2 is still live and serving production traffic.

To minimize downtime, cut DNS over last. Keep the v2 app serving production until the v3 app is fully verified, then repoint your custom domain's DNS (or the CNAME/ALIAS) to the v3 endpoint. DNS propagation means both racks may receive traffic briefly, so keep the v2 app running until traffic has drained, then decommission it. For custom domains and certificates on v3 see [Custom Domains](/deployment/custom-domains) and [SSL](/deployment/ssl).

## Gotchas and what is different

- **Build requires a Dockerfile.** v3 has no buildpack fallback. Add a `Dockerfile` for any service that lacked one on v2.
- **`command` is a string, not a list.** Convert any list-form commands.
- **Agent ports moved.** `agent.ports` is rejected on v3; declare `ports:` on the service and set `agent: true`.
- **Cron is 5-field.** Drop the trailing year field and `?` placeholders from AWS-style schedules.
- **`links` is gone.** Use service discovery by name plus shared `resources:`.
- **NLB blocks are gone.** Per-service `nlb:` is replaced by `balancers:` and `ports:`.
- **Process counts.** A static `scale.count` in the manifest is applied on first deploy only; later changes are made with `convox scale`. This matches v2 behavior but is worth re-confirming after migration.
- **The export/import bundle is not a reliable engine-to-engine path.** Map the manifest over instead.

## See Also

- [convox.yml](/configuration/convox-yml) for the full manifest reference
- [App Definition](/configuration/app-definition) for environment, volumes, agents, config mounts, and app settings
- [Service](/reference/primitives/app/service) for the full service field reference
- [Resource](/reference/primitives/app/resource) for datastore types and options
- [Timer](/reference/primitives/app/timer) for scheduled jobs
- [Autoscaling](/configuration/scaling/autoscaling) for v3 scaling and the new `scale.autoscale` block
- [Deploying Changes](/deployment/deploying-changes) for the deploy flow
- [Dockerfile](/configuration/dockerfile) for the build requirement
