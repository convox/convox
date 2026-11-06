---
title: "Migrating from Fly.io"
description: "Move a Fly.io app to Convox on your own cloud by translating fly.toml processes, secrets, scaling, volumes, and datastores into one convox.yml."
slug: fly
url: /migration/fly
---

# Migrating from Fly.io

This guide is for teams running applications on Fly.io that want to move to Convox on their own cloud account (AWS, GCP, Azure, DigitalOcean) or on bare metal. Both platforms run containers from a Dockerfile, so the build itself maps over directly. The main work is translating `fly.toml` into a [`convox.yml`](/configuration/convox-yml), moving secrets, and re-pointing managed datastores.

The practical reasons to migrate are usually one of: you want the cluster to run inside your own cloud account, you want standard Kubernetes underneath instead of Fly Machines, or you want a single manifest that covers services, databases, scheduled jobs, and scaling in one place.

## Prerequisites

- A running Convox [Rack](/reference/primitives/rack). See [Installation](/installation/production-rack).
- The [`convox` CLI](/installation/cli) installed and logged in.
- A `Dockerfile` for your app. If your Fly app used CNB Buildpacks (the `[build] builder` key) rather than a Dockerfile, you will need to add one. See [Dockerfile](/configuration/dockerfile). If your Fly app already used `[build] dockerfile` or a prebuilt `[build] image`, you can reuse it as-is.

## Concept Mapping

| Fly.io concept | Convox equivalent |
| --- | --- |
| `app` name | The Convox [App](/reference/primitives/app) name (set with `convox apps create` / `-a`) |
| `[build] dockerfile` / `[build] image` | Service [`build`](/reference/primitives/app/service#build) or [`image`](/reference/primitives/app/service) |
| `[build] builder` (Buildpacks) | A `Dockerfile` (Convox builds from a Dockerfile) |
| `[processes]` (process groups) | One [Service](/reference/primitives/app/service) per process, each with its own `command` |
| `[http_service]` | A Service with a `port` behind the default rack load balancer |
| `[http_service] internal_port` | Service `port` |
| `[http_service] force_https` | Service [`tls.redirect`](/reference/primitives/app/service#tls) (default `true`) |
| `[[services]]` + `[[services.ports]]` (raw TCP/UDP) | Service `port` / `ports`, or a custom [Balancer](/reference/primitives/app/balancer) |
| `min_machines_running` / `auto_stop_machines` | [`scale.min` / `scale.max`](/configuration/scaling/autoscaling) (set `min: 0` for scale-to-zero) |
| `[[vm]] cpus` / `memory` | [`scale.cpu`](/reference/primitives/app/service#scale) (1000 units = 1 CPU) / `scale.memory` (MB) |
| `[env]` | [`environment`](/configuration/environment) (non-secret defaults) |
| `flyctl secrets set` | [`convox env set`](/configuration/environment) |
| Fly Postgres / managed datastore | A Convox [Resource](/reference/primitives/app/resource) or external DB |
| `[[mounts]]` (Fly Volumes) | A [Volume](/configuration/volumes) |
| `[deploy] release_command` | Service [`initContainer`](/reference/primitives/app/service#initcontainer) or [`convox run`](/management/run) |
| Scheduled work (supercronic / scheduled Machines) | A [Timer](/reference/primitives/app/timer) |
| Internal-only service (no public handler) | Service [`internal: true`](/reference/primitives/app/service) |

## convox.yml

### Before: fly.toml

```toml
app = "myapp"
primary_region = "ord"

[build]
  dockerfile = "Dockerfile"

[env]
  RAILS_ENV = "production"
  LOG_LEVEL = "info"

[processes]
  web = "bundle exec rails server -b 0.0.0.0"
  worker = "bundle exec sidekiq"

[http_service]
  internal_port = 3000
  force_https = true
  auto_stop_machines = "stop"
  auto_start_machines = true
  min_machines_running = 2
  processes = ["web"]

[[vm]]
  size = "shared-cpu-1x"
  memory = "512mb"
  cpus = 1
```

Secrets on Fly are set out of band:

```bash
flyctl secrets set SECRET_KEY_BASE=... DATABASE_URL=...
```

### After: convox.yml

```yaml
environment:
  - RAILS_ENV=production
  - LOG_LEVEL=info
services:
  web:
    build: .
    command: bundle exec rails server -b 0.0.0.0
    port: 3000
    scale:
      count: 2
      cpu: 250
      memory: 512
  worker:
    build: .
    command: bundle exec sidekiq
    scale:
      count: 1
      cpu: 250
      memory: 512
```

Notes on the translation:

- Each Fly `[processes]` entry becomes its own Convox [Service](/reference/primitives/app/service) with the same `command`. Both share the one `build: .` because Fly runs every process group from the same image.
- The `web` service gets a `port`, which puts it behind the rack load balancer and terminates TLS automatically. The `worker` has no `port`, so it is not exposed, which matches a Fly process group with no service mapping.
- `force_https = true` is the Convox default ([`tls.redirect`](/reference/primitives/app/service#tls)), so you do not need to set anything for it.
- `min_machines_running = 2` becomes `scale.count: 2`. To reproduce Fly's `auto_stop_machines` scale-to-zero behavior, use [`scale.min: 0`](/configuration/scaling/autoscaling#scale-to-zero) with an autoscale trigger instead of a static count.
- `[[vm]]` sizing maps to `scale.cpu` (in CPU units, where `1000` is one full CPU) and `scale.memory` (in MB). A Fly `shared-cpu-1x` with `512mb` is roughly `cpu: 250`, `memory: 512`; size these against observed usage rather than the Fly preset name.

> The static `count` in `convox.yml` is only applied on the first deploy. After that, change replica counts with `convox scale` or an autoscale block. See [Autoscaling](/configuration/scaling/autoscaling).

## Environment and Secrets

Fly splits configuration into `[env]` (plaintext, in `fly.toml`) and secrets (`flyctl secrets set`, stored encrypted and never written to the file). Convox treats both as [environment variables](/configuration/environment); the difference is where the value lives.

- Non-secret values from `[env]` go into the `environment:` block of `convox.yml` with inline defaults, for example `- LOG_LEVEL=info`.
- Secret values that were set with `flyctl secrets set` are declared by name (no value) in `convox.yml` and have their values set with `convox env set`:

```yaml
environment:
  - SECRET_KEY_BASE
  - STRIPE_API_KEY
```

```bash
$ convox env set SECRET_KEY_BASE=... STRIPE_API_KEY=... -a myapp
Setting SECRET_KEY_BASE, STRIPE_API_KEY... OK
Release: RABCDEFGHI
```

Setting environment variables creates a new [Release](/reference/primitives/app/release). Promote it (or run `convox deploy`) to apply the change. Declaring a variable name with no default makes it required before a release can promote, which is a useful guard against shipping with a missing secret.

To list what Fly currently has set, run `flyctl secrets list` (names only) and `flyctl config show` (for `[env]`), then re-create those values with `convox env set`.

## Datastores

Fly Postgres (and other Fly datastores) are separate apps that hand your app a connection string, usually as the `DATABASE_URL` secret. On Convox you have two options.

**Run the datastore as a Convox Resource.** Declare it in `convox.yml` and link it to the services that need it. Convox injects connection environment variables based on the resource name:

```yaml
resources:
  database:
    type: postgres
services:
  web:
    build: .
    port: 3000
    resources:
      - database
  worker:
    build: .
    command: bundle exec sidekiq
    resources:
      - database
```

A `postgres` resource named `database` injects `DATABASE_URL`, `DATABASE_USER`, `DATABASE_PASS`, `DATABASE_HOST`, `DATABASE_PORT`, and `DATABASE_NAME`. For production on AWS you can switch the same resource to a managed RDS instance with `type: rds-postgres`, without changing application code. See [Resource](/reference/primitives/app/resource) for the full list of types (`postgres`, `mysql`, `mariadb`, `redis`, `memcached`, and their `rds-` / `elasticache-` managed variants) and the [overlay pattern](/reference/primitives/app/resource#overlays) for using containerized databases in dev and managed databases in production.

**Keep an external database.** If you are migrating data into an existing managed database outside the rack, or want to point at [Convox Cloud Databases](/cloud/databases), set the connection URL directly as an environment variable and do not declare a matching resource:

```bash
$ convox env set DATABASE_URL=postgres://user:pass@host:5432/dbname -a myapp
```

If you set an environment variable that matches a resource's injected URL (for example `DATABASE_URL` for a resource named `database`), Convox will not start the containerized resource and your service uses the external endpoint instead. See [Resource Overlays](/reference/primitives/app/resource#overlays).

To move the actual data, dump from Fly Postgres with `pg_dump` (connect through `flyctl proxy` to reach the Fly database) and load into the target. For a Convox Resource you can load through [`convox resources import`](/reference/primitives/app/resource#importing-data-to-a-resource) or by proxying to it with [`convox resources proxy`](/reference/primitives/app/resource#starting-a-proxy-to-a-resource).

## Scheduled Jobs and Workers

Fly has no scheduled-job key in `fly.toml`. Recurring work is typically run either by a long-lived worker process group (a `[processes]` entry) or by a cron tool such as supercronic baked into the image. Convox replaces both patterns with first-class primitives.

**Long-lived workers** map to a worker [Service](/reference/primitives/app/service) with no `port`, exactly like the `worker` example above.

**Recurring/cron jobs** map to a [Timer](/reference/primitives/app/timer). A Timer runs a command on a cron schedule against a named service. You can point it at an existing service or at a small template service scaled to zero so it only consumes resources when the job runs:

```yaml
services:
  worker:
    build: .
    command: bundle exec sidekiq
  jobs:
    build: .
    scale:
      count: 0
timers:
  nightly-cleanup:
    command: bin/cleanup
    schedule: "0 3 * * *"
    service: jobs
```

`schedule` uses standard cron syntax and all times are UTC. See [Timer](/reference/primitives/app/timer) for the full attribute set, including `concurrency` and `parallelCount`.

## Deploy and Cutover

1. Add `convox.yml` (and a `Dockerfile` if you were on Buildpacks) to your repository.
2. Create the app and set secrets before the first deploy so required variables exist:

   ```bash
   $ convox apps create myapp
   $ convox env set SECRET_KEY_BASE=... DATABASE_URL=... -a myapp
   ```

3. Build and deploy. Use `convox deploy` to build, create a [Release](/reference/primitives/app/release), and promote it in one step:

   ```bash
   $ convox deploy -a myapp
   ```

   If you need to run a one-off migration before traffic shifts (the equivalent of Fly's `[deploy] release_command`), split it into two steps so you can run the migration against the new release before promoting:

   ```bash
   $ convox build -a myapp
   $ convox run web bin/migrate -a myapp
   $ convox releases promote RBCDEFGHIJ -a myapp
   ```

   You can also run migrations automatically on every deploy with a service [`initContainer`](/reference/primitives/app/service#initcontainer). See [Deploying Changes](/deployment/deploying-changes).

4. Get the Convox URL for the web service and verify the app end to end while Fly is still serving production traffic:

   ```bash
   $ convox services -a myapp
   SERVICE  DOMAIN                                PORTS
   web      web.myapp.0a1b2c3d4e5f.convox.cloud   443:3000
   ```

5. **Cut over DNS last.** Point your custom domain at the Convox load balancer only after the app is verified healthy on Convox. Until DNS changes, Fly continues to serve traffic, so there is no downtime window during the migration itself. After DNS has propagated and traffic has drained from Fly, scale the Fly app down and decommission it. For attaching custom domains to a Convox service, see [Networking](/configuration/networking).

## Gotchas / What Is Different

- **Regions.** Fly's `primary_region` and edge placement have no direct equivalent. A Convox rack runs in one cloud region; for multi-region you run multiple racks. There is no `primary_region` key in `convox.yml`.
- **One image, many services.** Fly process groups all run the same image and differ only by command. Convox can do the same (`build: .` on each service), but it can also build different services from different paths or Dockerfiles. There is no shared global `command`; each service sets its own.
- **Scale-to-zero is opt-in.** Fly stops idle Machines by default with `auto_stop_machines`. Convox runs a static `count` unless you configure `scale.min: 0` with an autoscale trigger. See [Scale to Zero](/configuration/scaling/autoscaling#scale-to-zero).
- **Internal vs public.** On Fly, a process group is public only if it has a `[http_service]` or `[[services]]` mapping. On Convox, a service is public if it has a `port`. Use [`internal: true`](/reference/primitives/app/service) to keep a service with a port reachable only inside the rack.
- **TLS.** Convox provisions and renews certificates for service domains automatically and redirects HTTP to HTTPS by default. You do not configure handlers the way Fly does with `["tls", "http"]`.
- **Process listen address.** Make sure your app binds to `0.0.0.0` and the `port` you declare (Convox sets the `PORT` environment variable), the same requirement Fly has with `internal_port`.
- **Persistent volumes are per-replica.** A Fly Volume attaches to one Machine. A Convox [Volume](/configuration/volumes) attaches per process; if you need shared read-write storage across replicas on AWS, use the EFS-backed `volumeOptions.awsEfs` option rather than assuming a single shared disk.

## See Also

- [convox.yml](/configuration/convox-yml) for the full manifest reference
- [Service](/reference/primitives/app/service) for service attributes
- [Resource](/reference/primitives/app/resource) for databases and caches
- [Timer](/reference/primitives/app/timer) for scheduled jobs
- [Environment Variables](/configuration/environment) for secrets and configuration
- [Autoscaling](/configuration/scaling/autoscaling) for scale and scale-to-zero
- [Deploying Changes](/deployment/deploying-changes) for the deploy flow
- [Dockerfile](/configuration/dockerfile) for building from a Dockerfile
