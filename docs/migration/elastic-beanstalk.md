---
title: "Migrating from AWS Elastic Beanstalk"
slug: elastic-beanstalk
url: /migration/elastic-beanstalk
---
# Migrating from AWS Elastic Beanstalk

This guide is for teams running web and worker tiers on AWS Elastic Beanstalk who want a faster multi-service deploy workflow without leaving AWS. You stay on the same AWS account. Convox installs an EKS-based [Rack](/reference/primitives/rack) into that account, and you describe your whole application, multiple services, scheduled jobs, and datastores, in a single [`convox.yml`](/configuration/convox-yml). Where Beanstalk gives you one application version per environment and a separate worker environment for background jobs, Convox runs every service, timer, and resource for an app from one manifest and one `convox deploy`.

## Prerequisites

- A Convox [Rack](/reference/primitives/rack) installed into your AWS account. See [AWS installation](/installation/production-rack/aws).
- The [`convox` CLI](/installation/cli) installed and logged in.
- A `Dockerfile` for each service. Elastic Beanstalk's Docker platform builds from a `Dockerfile` (or pulls a prebuilt image referenced in `Dockerrun.aws.json`), so most projects already have one. If your application ran on a non-Docker Beanstalk platform (for example the native Python, Node.js, or Ruby platforms), you will need to add a `Dockerfile`. See [Dockerfile](/configuration/dockerfile).

## Concept Mapping

| Elastic Beanstalk | Convox |
|-------------------|--------|
| Application | [App](/reference/primitives/app) |
| Environment (web tier) | A [Rack](/reference/primitives/rack) plus a web [Service](/reference/primitives/app/service) with a `port` |
| Worker environment (SQS daemon tier) | A worker [Service](/reference/primitives/app/service) (no `port`) |
| `Dockerrun.aws.json` / Docker platform | `Dockerfile` plus a service entry in [`convox.yml`](/configuration/convox-yml) |
| Multi-container `Dockerrun.aws.json` v2 | Multiple services in one [`convox.yml`](/configuration/convox-yml) |
| Environment properties | [Environment variables](/configuration/environment) |
| `.ebextensions/*.config` | [`convox.yml`](/configuration/convox-yml) plus [rack parameters](/configuration/rack-parameters/aws) |
| `cron.yaml` periodic tasks | [Timers](/reference/primitives/app/timer) |
| RDS attached to an environment | A [Resource](/reference/primitives/app/resource) (`rds-postgres`, etc.) or an external/imported RDS |
| ElastiCache | An `elasticache-redis` / `elasticache-memcached` [Resource](/reference/primitives/app/resource) |
| Health check URL | Service [`health`](/reference/primitives/app/service#health) path |
| `eb deploy` | `convox deploy` |
| Rolling deployments | [Rolling updates](/deployment/rolling-updates) (default) |

## convox.yml

The example below covers a common Beanstalk setup: a web tier serving HTTP and a separate worker environment that drains an SQS queue, with a `cron.yaml` periodic task.

### Before (Elastic Beanstalk)

A multi-container web environment defined with `Dockerrun.aws.json` v2:

```json
{
  "AWSEBDockerrunVersion": 2,
  "containerDefinitions": [
    {
      "name": "web",
      "image": "myorg/web:latest",
      "essential": true,
      "memory": 512,
      "portMappings": [
        { "hostPort": 80, "containerPort": 3000 }
      ]
    }
  ]
}
```

Environment properties were set on the environment (console, `eb setenv`, or an `.ebextensions` `option_settings` block under `aws:elasticbeanstalk:application:environment`).

A separate worker environment ran the background processor, and its `cron.yaml` queued a periodic task:

```yaml
version: 1
cron:
  - name: "nightly-cleanup"
    url: "/tasks/cleanup"
    schedule: "0 3 * * *"
```

### After (convox.yml)

One manifest describes the web service, the worker service, and the scheduled job:

```yaml
environment:
  - DATABASE_POOL=10
services:
  web:
    build: .
    command: bin/web
    port: 3000
    health: /health
    environment:
      - SECRET_KEY_BASE
    resources:
      - database
  worker:
    build: .
    command: bin/worker
    environment:
      - SECRET_KEY_BASE
    resources:
      - database
resources:
  database:
    type: postgres
timers:
  nightly-cleanup:
    schedule: "0 3 * * *"
    command: bin/cleanup
    service: worker
```

Notes:

- `port` makes `web` reachable through the rack's load balancer. A service with no `port`, like `worker`, runs as a background process and is not exposed externally, which is the equivalent of a Beanstalk worker tier.
- `health` replaces the Beanstalk health check URL. See [Health Checks](/configuration/health-checks).
- The `command` overrides the image `CMD`, so the same `Dockerfile` can back both `web` and `worker`. If your worker uses a different build, point its `build` at another directory.

## Environment and Secrets

Beanstalk environment properties map directly to Convox [environment variables](/configuration/environment). Declare the variable names in `convox.yml` and set their values with the CLI:

```bash
$ convox env set SECRET_KEY_BASE=$(cat secret) DATABASE_POOL=10 -a myapp
Setting DATABASE_POOL, SECRET_KEY_BASE... OK
Release: RABCDEFGHI
```

Setting environment variables creates a new [Release](/reference/primitives/app/release). Promote it to apply the change:

```bash
$ convox releases promote RABCDEFGHI -a myapp
```

Differences worth noting:

- A variable declared at the top level of `convox.yml` is available to every service, like an environment-wide property. A variable declared under a single service is scoped to that service only.
- You can set a default in the manifest (`DATABASE_POOL=10`); a bare name (`SECRET_KEY_BASE`) must be set with `convox env set` before the release will promote.
- Convox stores values as Kubernetes Secrets in your cluster. There is no separate "save and apply" step in a console; the change rides along with the release.

## Datastores

If your Beanstalk environment provisioned an RDS instance, you have two clean paths.

### Let Convox manage a new database

Define a managed RDS [Resource](/reference/primitives/app/resource) and link it to the services that need it. Convox provisions the RDS instance in your AWS account and injects a connection URL:

```yaml
resources:
  database:
    type: rds-postgres
    options:
      class: db.t3.medium
      storage: 100
      version: "16"
      encrypted: true
      durable: true
services:
  web:
    resources:
      - database
```

Linking injects `DATABASE_URL` (and the broken-out `DATABASE_HOST`, `DATABASE_USER`, etc.) into the service. See the [Resource](/reference/primitives/app/resource) reference for the full variable list and for [containerized resources](/reference/primitives/app/resource#types) you can use in development.

### Keep your existing RDS instance

If you would rather keep the database that Beanstalk created, you have two options:

- **Import it** so Convox manages it going forward, using the `import` option with `masterUserPassword`. See [Database Import](/reference/primitives/app/resource#database-import).
- **Point at it directly** by setting the resource's connection environment variable yourself. Setting `DATABASE_URL` on the app stops Convox from starting a containerized `database` resource and connects the service straight to your existing RDS endpoint. See [Overlays](/reference/primitives/app/resource#overlays).

```bash
$ convox env set DATABASE_URL=postgres://user:pass@my-db.abc123.us-east-1.rds.amazonaws.com:5432/app -a myapp
```

ElastiCache maps the same way using the `elasticache-redis` and `elasticache-memcached` resource types. Convox Cloud also offers [managed databases](/cloud/databases) if you do not want to manage RDS yourself.

## Scheduled Jobs and Workers

Beanstalk splits background work into a separate worker environment with an SQS daemon, where periodic tasks are declared in `cron.yaml` and delivered as HTTP POSTs to a local endpoint. Convox handles the two responsibilities separately and more directly:

- **Continuous background processing** (the SQS daemon equivalent) is a worker [Service](/reference/primitives/app/service): a service with no `port` that runs your queue consumer as a long-lived process. It lives in the same app and manifest as the web service.
- **Periodic, scheduled tasks** (the `cron.yaml` equivalent) are [Timers](/reference/primitives/app/timer). A timer runs a `command` on a `schedule` against a named `service`:

```yaml
timers:
  nightly-cleanup:
    schedule: "0 3 * * *"
    command: bin/cleanup
    service: worker
```

A timer spawns a fresh [Process](/reference/primitives/app/process) of the named service for each run, so you do not need a queue, a daemon, or an HTTP endpoint to trigger scheduled work. Timer schedules use standard cron syntax and run in UTC. If a timer should run against a service that is otherwise idle, scale that service to zero and use it as a template; see [Using a Template Service](/reference/primitives/app/timer#using-a-template-service).

If your background work scales with queue depth rather than a fixed schedule, Convox can autoscale a worker service on a queue metric. See [Autoscaling](/configuration/scaling/autoscaling#event-driven-autoscaling-scaleautoscale).

## Deploy and Cutover

1. Add a `convox.yml` (and a `Dockerfile` if you did not have one) to your repository.
2. Create the app on the rack:

   ```bash
   $ convox apps create myapp
   ```

3. Set the environment variables your services require:

   ```bash
   $ convox env set SECRET_KEY_BASE=... DATABASE_POOL=10 -a myapp
   ```

4. Deploy. This builds your images, creates a [Release](/reference/primitives/app/release), and promotes it:

   ```bash
   $ convox deploy -a myapp
   ```

5. Find the rack-generated hostname for the web service and test it before changing any DNS:

   ```bash
   $ convox services -a myapp
   SERVICE  DOMAIN                                PORTS
   web      web.myapp.0a1b2c3d.convox.cloud       443:3000
   ```

   Exercise the app against that hostname (and migrate or restore your database if needed) while Beanstalk continues to serve production traffic.

6. **Cut over DNS last.** Once the Convox-hosted app is verified, point your domain at the Convox web service and attach your custom domain. See [Custom Domains](/deployment/custom-domains) and [SSL](/deployment/ssl). Because you only move DNS at the end, you can roll back by pointing DNS back at Beanstalk if anything looks wrong.

7. After traffic has shifted and you are confident, terminate the Beanstalk environments to stop paying for them.

To minimize downtime, keep both stacks running in parallel until DNS has fully propagated, and use a short TTL on the DNS record before the cutover so the switch takes effect quickly.

## Gotchas and Differences

- **No worker-tier SQS daemon.** Beanstalk's worker tier converts queue messages into HTTP POSTs to `localhost`. Convox does not. Run your existing queue consumer as a worker service that reads SQS directly, and use [Timers](/reference/primitives/app/timer) for anything you previously expressed in `cron.yaml`. The `cron.yaml` HTTP-POST pattern has no direct equivalent; the task logic moves into the timer's `command`.
- **`.ebextensions` does not carry over.** Platform and instance tuning that lived in `.ebextensions` is split in Convox: application shape goes in `convox.yml`, and cluster/platform settings (instance types, scaling, logging, networking) are [rack parameters](/configuration/rack-parameters/aws). There is no per-deploy shell-hook mechanism; use a service [`initContainer`](/reference/primitives/app/service#initcontainer) for migrations or setup that must run before the app starts.
- **Static `scale.count` is first-deploy only.** Unlike a Beanstalk environment where you adjust capacity in the console, a `count` in `convox.yml` is applied only on the first deploy. After that, change scale with `convox scale` or configure [autoscaling](/configuration/scaling/autoscaling).
- **One app, many services.** You do not create a separate environment per tier. Web and worker live in the same app and deploy together with one `convox deploy`. If you previously kept entirely separate Beanstalk applications, you can keep them separate as distinct Convox apps.
- **The load balancer is shared and managed.** Convox provisions and manages the rack load balancer for you. Service routing is driven by the `port` field rather than by a per-environment ELB you configure directly.
- **Logs and shell access.** Use `convox logs -a myapp` for application logs and `convox exec` / `convox run` instead of `eb logs` and `eb ssh`.

## See Also

- [convox.yml](/configuration/convox-yml) for the full manifest reference
- [Service](/reference/primitives/app/service) for service configuration including health checks and scale
- [Timer](/reference/primitives/app/timer) for scheduled jobs
- [Resource](/reference/primitives/app/resource) for managed and containerized datastores
- [Environment Variables](/configuration/environment) for configuration and secrets
- [Deploying Changes](/deployment/deploying-changes) for the build and promote workflow
- [AWS installation](/installation/production-rack/aws) for installing a Rack into your AWS account
