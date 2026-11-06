---
title: "Migrating from Render"
description: "Move a Render app to a Convox v3 rack by translating render.yaml services, env var groups, scaling, and managed datastores into one convox.yml."
slug: render
url: /migration/render
---
# Migrating from Render

This guide is for teams running services on Render that want to move to a Convox V3 rack on their own cloud account (AWS, GCP, Azure, DigitalOcean, or bare metal). The practical reasons to migrate are running in your own cloud account with your own networking and IAM, defining the whole app (services, datastores, scheduled jobs) in one `convox.yml`, and avoiding per-service plan tiers in favor of direct CPU and memory allocation. If you already deploy Render services from a Dockerfile, most of the work is translating `render.yaml` into `convox.yml`.

## Prerequisites

- A running Convox [Rack](/reference/primitives/rack) on your cloud provider.
- The [`convox` CLI](/reference/cli) installed and logged in.
- A [Dockerfile](/configuration/dockerfile) for each service. Render can build native (non-Docker) runtimes such as Node, Python, Ruby, and Go directly from source using a `buildCommand` and `startCommand`. Convox builds from a Dockerfile, so if your Render service used a native runtime rather than `runtime: docker`, you will need to add a Dockerfile that reproduces the same build and start steps. If your Render service already set `runtime: docker` with a `dockerfilePath`, you can reuse that Dockerfile as-is.

## Concept Mapping

| Render | Convox |
|--------|--------|
| `render.yaml` blueprint | [`convox.yml`](/configuration/convox-yml) manifest |
| `services:` entry with `type: web` | [Service](/reference/primitives/app/service) with a `port` |
| `services:` entry with `type: worker` | [Service](/reference/primitives/app/service) with no `port` |
| `services:` entry with `type: pserv` (private service) | [Service](/reference/primitives/app/service) with `internal: true` |
| `services:` entry with `type: cron` | [Timer](/reference/primitives/app/timer) |
| `services:` entry with `type: keyvalue` (managed Redis) | [Resource](/reference/primitives/app/resource) of `type: redis` |
| `databases:` (managed Postgres) | [Resource](/reference/primitives/app/resource) of `type: postgres` |
| `runtime: docker` + `dockerfilePath` | service `build` pointing at a [Dockerfile](/configuration/dockerfile) |
| Native `runtime` + `buildCommand` / `startCommand` | a Dockerfile plus the service `command` |
| `plan` (instance tier) | `scale.cpu` and `scale.memory` |
| `scaling` (min/max instances) | `scale.count` range plus `scale.targets` or `scale.autoscale` |
| `healthCheckPath` | service `health` |
| `envVars` (`key`/`value`) | service or app-level `environment` |
| `envVarGroups` + `fromGroup` | top-level `environment` shared across services |
| `envVars` with `fromDatabase` / `fromService` | resource linking (auto-injected connection variables) |
| Custom domain on a service | service `domain` |
| `disk` (persistent disk) | [Volume](/configuration/volumes) |

> Render occasionally renames blueprint keys between releases (for example, its managed Redis service `type` is now `keyvalue`). If your `render.yaml` uses older names, check the current Render Blueprint reference as you map fields.

## convox.yml

### Before (render.yaml)

```yaml
services:
  - type: web
    name: web
    runtime: docker
    dockerfilePath: ./Dockerfile
    plan: standard
    healthCheckPath: /healthz
    envVars:
      - key: RAILS_ENV
        value: production
      - key: DATABASE_URL
        fromDatabase:
          name: app-db
          property: connectionString
      - key: REDIS_URL
        fromService:
          type: keyvalue
          name: app-cache
          property: connectionString
      - fromGroup: app-secrets
    scaling:
      minInstances: 2
      maxInstances: 6
      targetCPUPercent: 70

  - type: worker
    name: worker
    runtime: docker
    dockerfilePath: ./Dockerfile
    dockerCommand: bin/worker
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: app-db
          property: connectionString
      - fromGroup: app-secrets

  - type: cron
    name: nightly-cleanup
    runtime: docker
    dockerfilePath: ./Dockerfile
    schedule: "0 3 * * *"
    dockerCommand: bin/cleanup
    envVars:
      - fromGroup: app-secrets

  - type: keyvalue
    name: app-cache
    ipAllowList: []

databases:
  - name: app-db
    plan: standard
    postgresMajorVersion: "16"

envVarGroups:
  - name: app-secrets
    envVars:
      - key: SECRET_KEY_BASE
        generateValue: true
```

### After (convox.yml)

```yaml
environment:
  - SECRET_KEY_BASE
  - RAILS_ENV=production
resources:
  database:
    type: postgres
  cache:
    type: redis
services:
  web:
    build: .
    port: 3000
    health: /healthz
    resources:
      - database
      - cache
    scale:
      count: 2-6
      targets:
        cpu: 70
  worker:
    build: .
    command: bin/worker
    resources:
      - database
timers:
  nightly-cleanup:
    schedule: "0 3 * * *"
    command: bin/cleanup
    service: worker
```

Notes on the translation:

- The Render `web` service becomes a service with a `port`. Set `port` to the port your app listens on. Render injects a `PORT` variable; Convox also sets [`PORT`](/configuration/environment#system-variables) to the value of your service `port`, so an app that already reads `PORT` keeps working.
- The Render `worker` (no inbound traffic) becomes a service with no `port` and an explicit `command`.
- The Render `cron` becomes a [Timer](/configuration/convox-yml#timers). A timer runs against an existing service; here it reuses the `worker` service image to run `bin/cleanup`. Render's `dockerCommand` for the cron maps to the timer `command`.
- `plan: standard` has no direct equivalent. Convox reserves resources per service with `scale.cpu` (1000 units is one full CPU) and `scale.memory` (MB). Pick values that match your previous plan's CPU and RAM. See [Autoscaling](/configuration/scaling/autoscaling).

## Environment and Secrets

Render distinguishes plain `envVars` (`key`/`value`), secret values (`sync: false` or `generateValue: true`), and shared `envVarGroups` attached with `fromGroup`. Convox handles all of these through [environment variables](/configuration/environment):

- A variable with a fixed value becomes a defaulted entry, for example `- RAILS_ENV=production`.
- A secret becomes a bare name in the manifest (`- SECRET_KEY_BASE`) whose value you set at deploy time and never commit:

```bash
$ convox env set SECRET_KEY_BASE=$(openssl rand -hex 64) -a myapp
Setting SECRET_KEY_BASE... OK
Release: RABCDEFGHI
```

- A Render `envVarGroup` shared across services maps to the **top-level** `environment` block, which is available to every service. Per-service `envVars` map to a service-level `environment` block. See [Application Level vs Service Level](/configuration/environment#defining-environment-variables).

Setting or changing an environment variable creates a new [Release](/reference/primitives/app/release); promote it to apply the change. There is no separate "environment group" object to manage in Convox; the manifest and `convox env set` are the only two places values live.

## Datastores

Render-managed Postgres (`databases:`) and managed key-value/Redis (`type: keyvalue`) become Convox [Resources](/reference/primitives/app/resource). Declare the resource and link it to the services that need it:

```yaml
resources:
  database:
    type: postgres
  cache:
    type: redis
services:
  web:
    build: .
    port: 3000
    resources:
      - database
      - cache
```

Linking a resource injects connection environment variables named after the resource. A resource named `database` injects `DATABASE_URL` (plus `DATABASE_HOST`, `DATABASE_USER`, `DATABASE_PASS`, `DATABASE_PORT`, `DATABASE_NAME`). This replaces Render's `fromDatabase` / `fromService` references: you no longer wire `DATABASE_URL` by hand because linking produces it. If your code reads `DATABASE_URL` and `REDIS_URL`, name the resources so the injected variable matches, or set an explicit env var to the injected value.

Production options:

- The example above starts **containerized** Postgres and Redis inside the rack. That is the fastest path and good for staging.
- For production durability on AWS, switch to managed services with a [Resource Overlay](/reference/primitives/app/resource#overlays): change `type: postgres` to `type: rds-postgres` and `type: redis` to `type: elasticache-redis`. The injected `DATABASE_URL` keeps the same format, so application code does not change. See [PostgreSQL](/reference/primitives/app/resource/postgres) and [Redis](/reference/primitives/app/resource/redis).
- On [Convox Cloud](/cloud/databases) you can use managed databases with simplified pricing.

To migrate existing data, export from Render (for Postgres, `pg_dump` against the Render external connection string) and import into the Convox resource with [`convox resources import`](/reference/primitives/app/resource#importing-data-to-a-resource). You can also keep an external managed database by setting its connection string directly with `convox env set DATABASE_URL=...`, which stops Convox from starting a containerized resource of that name (see [Overlays](/reference/primitives/app/resource#overlays)).

## Scheduled Jobs and Workers

| Render | Convox |
|--------|--------|
| `type: cron` with `schedule` | [Timer](/reference/primitives/app/timer) with `schedule` and `command` |
| `type: worker` (long-running background process) | [Service](/reference/primitives/app/service) with no `port` |

A Render cron job runs a command on a schedule. A Convox timer does the same, but it runs against an existing service rather than being its own deployable unit. You can point a timer at any service, including one scaled to zero, which is a clean way to provide a job runner image:

```yaml
services:
  jobs:
    build: ./jobs
    scale:
      count: 0
timers:
  nightly-cleanup:
    schedule: "0 3 * * *"
    command: bin/cleanup
    service: jobs
    concurrency: Forbid
```

Render's `schedule` uses standard cron syntax and so does Convox. All Convox timer schedules run in UTC. Set `concurrency: Forbid` if a run must never overlap a previous run. See [Timer](/reference/primitives/app/timer) for the full attribute list.

Render background workers (`type: worker`) map directly to a Convox service with no `port`. Give it an explicit `command` and link any resources it needs.

## Deploy and Cutover

1. Add a [Dockerfile](/configuration/dockerfile) for each service if you used a native Render runtime, and write the `convox.yml` above.
2. Create the app on your rack:

```bash
$ convox apps create myapp
```

3. Set secrets that the manifest declares as bare names:

```bash
$ convox env set SECRET_KEY_BASE=... -a myapp
```

4. Migrate datastore data (see [Datastores](#datastores)) before sending traffic.
5. Deploy:

```bash
$ convox deploy -a myapp
```

This builds, creates a [Release](/reference/primitives/app/release), and promotes it as a [rolling deployment](/deployment/rolling-updates). See [Deploying Changes](/deployment/deploying-changes).

6. Verify the new deployment is healthy before changing DNS:

```bash
$ convox ps -a myapp
$ convox services -a myapp
```

Test against the Convox-assigned service hostname shown by `convox services`.

7. **Cut over DNS last.** Keep the Render service running, attach your custom domain to the Convox service with the `domain` attribute, then move your DNS record to the Convox load balancer. Once traffic has drained from Render and you have confirmed the Convox app is serving production traffic, decommission the Render service. Doing the DNS change last keeps a working rollback target during the cutover.

## Gotchas

- **No native buildpacks.** Render builds native runtimes from source with `buildCommand` and `startCommand`. Convox always builds from a Dockerfile. Reproduce the build steps in a Dockerfile and put the start command in the service `command` (or the Dockerfile `CMD`).
- **Plans become explicit resources.** There is no `plan: standard`. You set CPU and memory directly with `scale.cpu` and `scale.memory`. Translate your old plan's CPU/RAM into those numbers.
- **Connection strings come from linking, not references.** Render's `fromDatabase` / `fromService` wiring is replaced by linking a resource to a service, which auto-injects the connection variables. Do not also hand-set those variables unless you are intentionally pointing at an external datastore.
- **The `PORT` contract.** Convox sets `PORT` to your service's `port` value. If your app binds to a hardcoded port, set the service `port` to match it.
- **Static sites.** Render's `type: static` has no direct Convox primitive. Serve static assets from a small web service (for example an `nginx` Dockerfile) behind a `port`, or host them on object storage and a CDN outside Convox.
- **Private services.** A Render `pserv` (private service) maps to a service with `internal: true`, reachable only inside the rack. See [Service Discovery](/configuration/service-discovery).
- **Health checks.** Render's single `healthCheckPath` maps to the readiness `health` path. Convox also supports separate liveness and startup probes; see [Health Checks](/configuration/health-checks).

## See Also

- [convox.yml](/configuration/convox-yml) for the full manifest reference
- [Service](/reference/primitives/app/service) for ports, scaling, and health checks
- [Resource](/reference/primitives/app/resource) for databases and caches, including managed overlays
- [Timer](/reference/primitives/app/timer) for scheduled jobs
- [Environment Variables](/configuration/environment) for configuration and secrets
- [Deploying Changes](/deployment/deploying-changes) for the deploy and promote flow
- [Dockerfile](/configuration/dockerfile) for building services from source
