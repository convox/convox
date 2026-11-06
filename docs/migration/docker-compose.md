---
title: "Migrating from Docker Compose"
description: "Move a Docker Compose app to Convox by translating compose.yaml services, ports, environment, volumes, and datastores into one convox.yml on a Rack."
slug: migrating-from-docker-compose
url: /migration/docker-compose
---

# Migrating from Docker Compose

This guide is for teams running a `compose.yaml` (or `docker-compose.yml`) locally or on a single host who want to move to a managed Kubernetes deployment without rewriting their containers. Docker Compose and Convox describe the same building blocks, services, images, ports, environment, and dependencies, so most of the migration is a mechanical translation from one YAML file to another. Convox then runs those services on a [Rack](/reference/primitives/rack) with rolling deploys, autoscaling, load balancing, and managed datastores that Compose does not provide.

## Prerequisites

- A Convox [Rack](/reference/primitives/rack) on the provider of your choice, and the `convox` CLI installed and logged in.
- A `Dockerfile` for each service you build from source. Compose can build from a `Dockerfile`, so if your `build:` blocks already point at one you are set. If a service used a prebuilt `image:` you can keep using it. See [Dockerfile](/configuration/dockerfile).

## Concept Mapping

| Docker Compose | Convox equivalent |
| -------------- | ----------------- |
| `services:` | [`services:`](/reference/primitives/app/service) in `convox.yml` |
| `build:` (string or `context`/`dockerfile`) | [`build:`](/reference/primitives/app/service#build) (`path` and `manifest`) |
| `image:` | [`image:`](/reference/primitives/app/service) |
| `command:` | [`command:`](/reference/primitives/app/service) |
| `ports:` host:container mapping | [`port:`](/configuration/load-balancers) (the container port; the Rack assigns the public address) |
| `expose:` / internal ports | [`internal: true`](/configuration/service-discovery#internal-services) plus the service hostname |
| `environment:` | [`environment:`](/configuration/environment) |
| `env_file:` | [`convox env set`](/configuration/environment#setting-environment-variables) (values live on the Release, not in the repo) |
| named `volumes:` | [Volumes](/configuration/volumes) (`volumes:` / `volumeOptions:`, for persistence) |
| `volumes:` host bind mount | Not supported (see [Gotchas](#gotchas-and-what-is-different)) |
| `depends_on:` and inter-service networking | [Service discovery](/configuration/service-discovery) by hostname; ordering via [`initContainer`](/reference/primitives/app/service#initcontainer) |
| database/cache service (postgres, redis, ...) | [Resources](/reference/primitives/app/resource) |
| `deploy.replicas:` | [`scale.count`](/configuration/scaling/autoscaling) |
| `restart:` policy | Automatic; Convox restarts failed [Processes](/reference/primitives/app/process) |
| `healthcheck:` | [Health checks](/configuration/health-checks) (`health:`, `liveness:`) |

## convox.yml

### Before: compose.yaml

```yaml
services:
  web:
    build: .
    command: bin/web
    ports:
      - "3000:3000"
    environment:
      - RAILS_ENV=production
      - SECRET_KEY_BASE
    depends_on:
      - db
      - cache
  worker:
    build: .
    command: bin/worker
    environment:
      - RAILS_ENV=production
      - SECRET_KEY_BASE
    depends_on:
      - db
      - cache
  db:
    image: postgres:16
    environment:
      - POSTGRES_PASSWORD=secret
    volumes:
      - db-data:/var/lib/postgresql/data
  cache:
    image: redis:7

volumes:
  db-data:
```

### After: convox.yml

```yaml
environment:
  - RAILS_ENV=production
  - SECRET_KEY_BASE
resources:
  db:
    type: postgres
  cache:
    type: redis
services:
  web:
    build: .
    command: bin/web
    port: 3000
    resources:
      - db
      - cache
  worker:
    build: .
    command: bin/worker
    resources:
      - db
      - cache
```

The two `postgres` and `redis` containers from Compose become Convox [Resources](/reference/primitives/app/resource). You no longer declare the database image, password, or its data volume by hand, Convox provisions the datastore and injects its connection string. The `web` and `worker` services share the same build (the same `Dockerfile`) and differ only by `command`, exactly as they did in Compose.

## Environment and Secrets

Compose reads `environment:` and `env_file:` at container start, often from values committed next to the source. In Convox, declare the variable names your services need in `convox.yml` and set their values on the App with `convox env set`. The values live on the [Release](/reference/primitives/app/release), not in your repository.

```yaml
environment:
  - RAILS_ENV=production
  - SECRET_KEY_BASE
```

A variable with a default (`RAILS_ENV=production`) is set in the manifest. A variable with no default (`SECRET_KEY_BASE`) must be supplied before you deploy:

```bash
$ convox env set SECRET_KEY_BASE=$(openssl rand -hex 64) -a myapp
Setting SECRET_KEY_BASE... OK
Release: RABCDEFGHI
```

Variables declared at the top level are available to every service; variables declared under a service apply only to that service. See [Environment Variables](/configuration/environment).

## Datastores

A `postgres`, `mysql`, `mariadb`, `redis`, or `memcached` service in your Compose file maps directly to a Convox [Resource](/reference/primitives/app/resource). Declare the resource, link it to the services that use it, and Convox runs the datastore and injects connection environment variables based on the resource name:

```yaml
resources:
  db:
    type: postgres
services:
  web:
    build: .
    port: 3000
    resources:
      - db
```

Linking the `db` resource injects `DB_URL`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, and `DB_NAME` into the `web` service. Point your app at `DB_URL`.

By default a Resource runs as a container inside the Rack, which is the closest match to a database container in Compose and is fine for development and staging. For production durability you can overlay the same resource onto a managed service (AWS RDS or ElastiCache, or a [Convox Cloud database](/cloud/databases)) without changing your application code:

```yaml
resources:
  db:
    type: rds-postgres
    options:
      class: db.t3.large
      storage: 100
      encrypted: true
```

The injected `DB_URL` keeps the same format, so the switch is transparent to your code. See [Resource](/reference/primitives/app/resource) for the full list of types, the managed overlays, and how to import an existing database. If your database already lives outside the cluster, set its connection string as an environment variable and skip the resource entirely.

## Scheduled Jobs, Workers, and Cron

Compose does not have a native scheduler, so cron-style work is usually a separate long-running container or a host cron job. Convox splits these into two cleaner primitives:

- **Long-running workers** (queue consumers, background processors) are plain [Services](/reference/primitives/app/service) with a `command` and no `port`, exactly like the `worker` service above.
- **Scheduled jobs** are [Timers](/reference/primitives/app/timer). A Timer runs a `command` against a named service on a cron schedule (UTC):

```yaml
services:
  jobs:
    build: ./jobs
    scale:
      count: 0
timers:
  nightly-cleanup:
    command: bin/cleanup
    schedule: "0 3 * * *"
    service: jobs
```

The `jobs` service can be scaled to zero so it costs nothing when idle; the Timer spins up a [Process](/reference/primitives/app/process) on schedule. See [Timer](/reference/primitives/app/timer).

## Deploy and Cutover

Once your `convox.yml` and `Dockerfile` are in place:

1. Set any environment variables that have no default in the manifest:

   ```bash
   $ convox env set SECRET_KEY_BASE=... -a myapp
   ```

2. Build and promote in one step:

   ```bash
   $ convox deploy -a myapp
   ```

   This packages your source, builds each service image, creates a [Release](/reference/primitives/app/release), and promotes it with a [rolling deployment](/deployment/rolling-updates). See [Deploying Changes](/deployment/deploying-changes).

3. Find the public address Convox assigned to each externally-routed service:

   ```bash
   $ convox services -a myapp
   SERVICE  DOMAIN                                PORTS
   web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:3000
   ```

4. Verify the new deployment over that hostname before sending real traffic. Run any one-off data migration with [`convox run`](/management/run) against the new Release.

**Minimize downtime by cutting DNS over last.** Deploy on Convox and validate it fully against the Convox-assigned hostname while your existing Compose host keeps serving production. Only after the Convox deployment is confirmed healthy do you point your custom domain at it (see [Custom Domains](/configuration/networking)). This keeps the old environment as a fallback until the cutover is complete.

## Gotchas and What Is Different

- **Host bind mounts do not carry over.** Compose `volumes:` entries like `./src:/app` mount a host directory into the container for live local development. Convox runs your code from the built image; there is no host filesystem to bind into. Convox [Volumes](/configuration/volumes) exist for *persistence* (EFS, Azure Files) and *scratch space* (`emptyDir`), not for sharing source from a developer's machine.
- **Host networking and published host ports are gone.** Compose `ports: "3000:3000"` publishes a port on the host. On Convox you declare only the container `port:` and the Rack provisions a load balancer and address for it. There is no host port mapping and no `network_mode: host`.
- **`depends_on` is not a startup gate by default.** Convox does not order service startup the way Compose `depends_on` with `condition: service_healthy` does. Services come up in parallel and reconnect once dependencies are ready. When a service genuinely must wait (for example, run migrations before booting), use an [`initContainer`](/reference/primitives/app/service#initcontainer) to block until a dependency is healthy or a task has completed.
- **Inter-service calls use hostnames, not Compose service-name links.** Compose lets one service reach another by its service name on a shared network. On Convox, reach another service by its [service discovery](/configuration/service-discovery) hostname (mark backend-only services `internal: true`).
- **Compose-only dev conveniences have no equivalent.** `develop`/`watch`, `profiles`, `extends`, and `restart:` policies are local-development or single-host features. Convox restarts failed [Processes](/reference/primitives/app/process) automatically and uses [Releases](/reference/primitives/app/release) and the CLI for the deploy workflow instead.
- **Replica counts move to `scale`.** Translate `deploy.replicas` to [`scale.count`](/configuration/scaling/autoscaling), and consider switching to autoscaling once you are running on a Rack.

## See Also

- [convox.yml](/configuration/convox-yml) for the full manifest reference
- [Service](/reference/primitives/app/service) for the complete list of service attributes
- [Resource](/reference/primitives/app/resource) for databases, caches, and managed overlays
- [Timer](/reference/primitives/app/timer) for scheduled jobs
- [Environment Variables](/configuration/environment) for managing configuration and secrets
- [Service Discovery](/configuration/service-discovery) for inter-service networking
- [Volumes](/configuration/volumes) for persistent and ephemeral storage
- [Deploying Changes](/deployment/deploying-changes) for the deploy and promote flow
