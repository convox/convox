---
title: "Migrating to Convox"
slug: migration
url: /migration
---
# Migrating to Convox

These guides walk through moving an existing application onto Convox. The goal of each guide is the same: take a config you already have (a `Procfile`, a `render.yaml`, a `fly.toml`, a `docker-compose.yml`, an Elastic Beanstalk environment, or a Convox v2 app) and produce an equivalent [`convox.yml`](/configuration/convox-yml) that runs on a Convox [Rack](/reference/primitives/rack) in your own cloud account.

Convox gives you a Heroku-style deploy flow (`convox deploy`, environment variables, releases, rollbacks) while the workloads run on Kubernetes infrastructure you own. Each guide maps the source platform's concepts to Convox primitives ([Services](/reference/primitives/app/service), [Resources](/reference/primitives/app/resource), [Timers](/reference/primitives/app/timer), and [Environment Variables](/configuration/environment)), shows a before/after config, and covers datastore and cutover steps.

## Before You Start

You need a Convox [Rack](/reference/primitives/rack) and the `convox` CLI. If your source platform built images from a buildpack rather than a `Dockerfile`, you will need to add one before deploying. See [Dockerfile](/configuration/dockerfile).

## Guides

| Migrate from | Use this if... |
|--------------|----------------|
| [Heroku](/migration/heroku) | You deploy with a `Procfile` and Heroku add-ons and want the same workflow on your own cloud. |
| [Render](/migration/render) | You define services in `render.yaml` and want to move off Render's managed platform. |
| [Fly.io](/migration/fly) | You run on Fly with a `fly.toml` and want a Kubernetes-backed deployment in your own account. |
| [Docker Compose](/migration/docker-compose) | You already have a `docker-compose.yml` and want to run those services in production on a Rack. |
| [AWS Elastic Beanstalk](/migration/elastic-beanstalk) | You run on Elastic Beanstalk and want container-based deploys with explicit infrastructure control. |
| [Convox v2 to v3](/migration/v2-to-v3) | You run a Convox v2 (CloudFormation/ECS) rack and want to move apps to a v3 (Kubernetes) rack. |

## See Also

- [convox.yml](/configuration/convox-yml) for the full manifest reference
- [Deploying Changes](/deployment/deploying-changes) for the build, release, and promote flow
- [Environment Variables](/configuration/environment) for setting secrets and config
- [Resource](/reference/primitives/app/resource) for databases and caches
