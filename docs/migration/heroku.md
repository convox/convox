---
title: "Migrating from Heroku"
slug: heroku
url: /migration/heroku
---
# Migrating from Heroku

This guide is for teams running an application on Heroku that want to move it to Convox without rewriting it. If you already deploy containers, the move is mostly translating your Procfile, config vars, and add-ons into a single [`convox.yml`](/configuration/convox-yml). If you deploy with buildpacks, you will add a `Dockerfile` first. You can cut over with minimal downtime by deploying to Convox, migrating data, and switching DNS last.

## Prerequisites

- A Convox [Rack](/reference/primitives/rack) and the `convox` CLI installed and pointed at it (`convox switch <rack>`).
- A `Dockerfile` if your app currently builds with Heroku buildpacks. Heroku buildpacks do not run on Convox; you containerize the app instead. See [Dockerfile](/configuration/dockerfile). If you already push prebuilt images to Heroku's container registry, you can reuse that image with the service `image:` field instead of `build:`.

## Concept Mapping

| Heroku | Convox |
| ------ | ------ |
| App | [App](/reference/primitives/app) |
| Buildpack | [Dockerfile](/configuration/dockerfile) |
| `web` process type (Procfile) | A [Service](/reference/primitives/app/service) with a `port` |
| `worker` / other process types (Procfile) | A [Service](/reference/primitives/app/service) with no `port` |
| Procfile command | The service `command` in `convox.yml` |
| Dyno (`heroku ps:scale web=3`) | Service `scale` plus `convox scale web --count 3` |
| Config vars | [Environment Variables](/configuration/environment) |
| Heroku Postgres add-on | A `postgres` (or `rds-postgres`) [Resource](/reference/primitives/app/resource) |
| Heroku Redis / Key-Value add-on | A `redis` (or `elasticache-redis`) [Resource](/reference/primitives/app/resource) |
| Heroku Scheduler | A [Timer](/reference/primitives/app/timer) |
| `heroku run <cmd>` | [`convox run`](/management/run) |
| Custom domain + ACM/automatic certs | Service `domain` plus a CNAME to the Rack router. See [Custom Domains](/deployment/custom-domains). |

## convox.yml

### Before: Heroku

A typical Heroku Rails app is described by a `Procfile` and config vars set with `heroku config:set`:

```text
# Procfile
web: bundle exec rails server -b 0.0.0.0 -p $PORT
worker: bundle exec sidekiq
release: bundle exec rails db:migrate
```

Add-ons are attached separately (`heroku-postgresql`, `heroku-redis`), and the connection strings arrive as the `DATABASE_URL` and `REDIS_URL` config vars. Scheduled work is configured in the Heroku Scheduler dashboard.

### After: convox.yml

The same app expressed for Convox. Each Procfile process type becomes a [Service](/reference/primitives/app/service), the `release` command becomes an [init container](/reference/primitives/app/service#initcontainer) that runs migrations before the web container starts, add-ons become [Resources](/reference/primitives/app/resource), and the scheduled job becomes a [Timer](/reference/primitives/app/timer):

```yaml
environment:
  - SECRET_KEY_BASE
  - RAILS_ENV=production
resources:
  database:
    type: postgres
    options:
      version: "16"
  cache:
    type: redis
services:
  web:
    build: .
    command: bundle exec rails server -b 0.0.0.0 -p 3000
    port: 3000
    health: /healthz
    scale:
      count: 2
      cpu: 250
      memory: 512
    resources:
      - database
      - cache
    initContainer:
      command: bundle exec rails db:migrate
  worker:
    build: .
    command: bundle exec sidekiq
    scale:
      count: 1
    resources:
      - database
      - cache
timers:
  daily-report:
    schedule: "0 6 * * *"
    command: bin/rake reports:daily
    service: worker
```

Notes on the translation:

- Heroku injects a dynamic `$PORT`; on Convox you bind to a fixed port and set `port:` to the same value. The web service's load balancer routes to that port.
- There is no Procfile `release` phase on Convox. Run release-time work such as migrations with an [`initContainer`](/reference/primitives/app/service#initcontainer), which runs to completion before the main container starts and receives the same environment and resource connections.
- A process type with no `port` (like `worker`) becomes a service with no `port` and gets no load balancer.

## Environment and Secrets

Heroku config vars map directly to Convox [Environment Variables](/configuration/environment).

Declare the variable names your app needs in `convox.yml` (top level for every service, or under a single service to scope it). Values can carry a default (`RAILS_ENV=production`) or be left bare (`SECRET_KEY_BASE`) to require they be set before deploy:

```yaml
environment:
  - SECRET_KEY_BASE
  - RAILS_ENV=production
```

Export your current Heroku config and set the values on Convox:

```bash
$ heroku config -s | tee heroku.env
$ convox env set SECRET_KEY_BASE=... -a myapp
Setting SECRET_KEY_BASE... OK
Release: RABCDEFGHI
```

Setting environment variables creates a new [Release](/reference/primitives/app/release) but does not deploy it. Promote it (or include the change in your next `convox deploy`) to make the values live. You do not need to copy `DATABASE_URL` or `REDIS_URL`; Convox injects those from the linked resources (see below).

## Datastores

Heroku add-ons that provide a database or cache become Convox [Resources](/reference/primitives/app/resource). Linking a resource to a service injects connection environment variables named after the resource. For a `postgres` resource named `database`, Convox sets:

```text
DATABASE_URL=postgres://username:password@host.name:port/database
DATABASE_USER=username
DATABASE_PASS=password
DATABASE_HOST=host.name
DATABASE_PORT=port
DATABASE_NAME=database
```

so a Rails app that reads `DATABASE_URL` works without changes. Map the common Heroku add-ons like this:

| Heroku add-on | Convox resource type |
| ------------- | -------------------- |
| Heroku Postgres | `postgres` (in-cluster) or `rds-postgres` (AWS managed) |
| Heroku Redis / Key-Value Store | `redis` (in-cluster) or `elasticache-redis` (AWS managed) |
| ClearDB / JawsDB MySQL | `mysql` (in-cluster) or `rds-mysql` (AWS managed) |

The in-cluster types (`postgres`, `redis`, `mysql`) are quick to start and good for development and staging. For production durability on AWS, use the managed `rds-` and `elasticache-` types, which add backups, encryption, and Multi-AZ options. The injected variable names are identical for both, so you can use an [overlay](/reference/primitives/app/resource#overlays) to run containerized in one environment and managed in another without code changes. On Convox Cloud, see [Convox Cloud Databases](/cloud/databases) for managed options. See [Resource](/reference/primitives/app/resource) for the full list of types and options.

## Scheduled Jobs and Workers

Heroku Scheduler jobs become Convox [Timers](/reference/primitives/app/timer). A timer runs a `command` against an existing service on a [cron](https://crontab.guru) schedule (UTC):

```yaml
timers:
  daily-report:
    schedule: "0 6 * * *"
    command: bin/rake reports:daily
    service: worker
```

A timer can target a service that is scaled to zero, which mirrors the Heroku one-off-dyno model: define a `jobs` service with `scale: { count: 0 }` and point timers at it so it only spins up on schedule. See [Using a Template Service](/reference/primitives/app/timer#using-a-template-service).

Long-running background workers (Sidekiq, Resque, Celery) map to a [Service](/reference/primitives/app/service) with no `port`, scaled by replica count, exactly like a Heroku worker dyno. Use `convox scale worker --count N` or, if traffic is variable, [autoscaling](/configuration/scaling/autoscaling).

## Deploy and Cutover

Deploy to Convox first and verify it before moving any traffic. DNS is the last step.

1. Create the app on the Rack:
   ```bash
   $ convox apps create myapp
   ```
2. Set the environment variables you exported from Heroku:
   ```bash
   $ convox env set SECRET_KEY_BASE=... -a myapp
   ```
3. Build and promote in one step. Convox creates a [Release](/reference/primitives/app/release) and starts a [rolling deployment](/deployment/rolling-updates):
   ```bash
   $ convox deploy -a myapp
   ```
   The `initContainer` runs your migrations before the web container starts. See [Deploying Changes](/deployment/deploying-changes).
4. Find the load balancer hostname and smoke-test the app before touching DNS:
   ```bash
   $ convox services -a myapp
   SERVICE  DOMAIN                                PORTS
   web      web.convox.0a1b2c3d4e5f.convox.cloud  443:3000
   ```
5. Migrate your data. Heroku writes are still live at this point, so do a final sync during the cutover window. Capture from Heroku and restore into the Convox resource over a proxy:
   ```bash
   $ heroku pg:backups:capture -a heroku-app
   $ heroku pg:backups:download -a heroku-app          # writes latest.dump
   $ convox resources proxy database -a myapp           # proxies localhost:5432
   $ pg_restore --verbose --clean --no-acl --no-owner \
       -h localhost -U <user> -d <name> latest.dump
   ```
   Get the connection credentials with `convox resources url database -a myapp`. If port 5432 is in use locally, stop your local Postgres or proxy on a different port.
6. Cut over DNS last to minimize downtime:
   - A day ahead, lower the TTL on the DNS record you are migrating (for example to 60 seconds) so the change propagates quickly.
   - Add the production hostname to the web service `domain` and redeploy:
     ```yaml
     services:
       web:
         domain: www.example.com
     ```
     Convox provisions a certificate automatically for the domain. See [Custom Domains](/deployment/custom-domains) and [SSL](/deployment/ssl).
   - At the cutover moment, disable Heroku Scheduler jobs, put the Heroku app in maintenance mode, let any worker dynos drain, and scale them to zero so nothing else writes to the Heroku database.
   - Run a final data sync (step 5) to catch writes since the first restore.
   - Point the production CNAME at the Rack router. Find it with `convox rack`:
     ```bash
     $ convox rack
     Name      convox
     Provider  aws
     Router    router.0a1b2c3d4e5f.convox.cloud
     Status    running
     ```
     Then set `www.example.com CNAME router.0a1b2c3d4e5f.convox.cloud`.
   - Wait for DNS to propagate and verify the site serves from Convox.

## Gotchas and Differences

- **No `$PORT` injection.** Heroku assigns a port at runtime; on Convox you choose a fixed port and declare it as `port:`. Convox sets a `PORT` environment variable, but the value is your declared port, not a dynamic one.
- **No Procfile `release` phase.** Migrations and other release-time tasks run in an [`initContainer`](/reference/primitives/app/service#initcontainer) (runs before the service starts) or as a one-off with [`convox run`](/management/run).
- **No buildpacks.** Convox builds from a `Dockerfile`. Pin your runtime, install system packages explicitly, and set `CMD` (or override it with the service `command`).
- **Ephemeral filesystem, like Heroku.** Containers have an ephemeral disk. If you need persistence, use a [Resource](/reference/primitives/app/resource) (object storage, database) or a [Volume](/configuration/volumes); do not write durable data to the local filesystem.
- **`convox.yml` is the source of truth.** Scale, schedules, domains, and env var names live in `convox.yml` and are versioned with your code, rather than being configured imperatively in a dashboard. The static `scale.count` applies only on first deploy; afterward, change running count with `convox scale`.
- **Logs and one-off commands.** Use `convox logs -a myapp` in place of `heroku logs --tail`, and `convox run <service> <cmd>` in place of `heroku run`.

## See Also

- [convox.yml](/configuration/convox-yml)
- [Service](/reference/primitives/app/service)
- [Resource](/reference/primitives/app/resource)
- [Timer](/reference/primitives/app/timer)
- [Environment Variables](/configuration/environment)
- [Deploying Changes](/deployment/deploying-changes)
- [Custom Domains](/deployment/custom-domains)
- [Autoscaling](/configuration/scaling/autoscaling)
</content>
</invoke>
