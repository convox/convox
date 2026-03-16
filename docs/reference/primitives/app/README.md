---
title: "App"
slug: app
url: /reference/primitives/app
---
# App

An App is a logical container for [Primitives](/reference/primitives) that are updated together through transactional deployments.

## Primitives

| Primitive | Description |
|:----------|:------------|
| [Balancer](/reference/primitives/app/balancer) | Custom TCP load balancers for non-HTTP protocols (e.g., raw TCP, gRPC). Routes external traffic to a Service on specific ports. |
| [Build](/reference/primitives/app/build) | A compiled snapshot of your codebase, produced from a Dockerfile. Each deploy creates a new Build. |
| [Object](/reference/primitives/app/object) | Blob/file storage for uploading and downloading files from your application. |
| [Process](/reference/primitives/app/process) | A running container instance. Processes are created from a Release and managed by a Service or `convox run`. |
| [Release](/reference/primitives/app/release) | A unit of deployment that pairs a Build with a set of environment variables. Promoting a Release deploys it. |
| [Resource](/reference/primitives/app/resource) | A network-accessible backing service such as PostgreSQL, MySQL, Redis, or Memcached. Can be containerized or cloud-managed. |
| [Service](/reference/primitives/app/service) | A horizontally-scalable group of durable Processes defined in `convox.yml`. Services are the primary workload primitive. |
| [Timer](/reference/primitives/app/timer) | A scheduled task that runs a command on a cron schedule using a Service's image. Maps to a Kubernetes CronJob. |

## App Definition

An App is defined by a single [`convox.yml`](/configuration/convox-yml)
```yaml
labels:
  convox.com/test: true
resources:
  database:
    type: postgres
services:
  web:
    build: .
    resources:
      - database
```
## App CLI Commands

### Creating an App
```bash
    $ convox apps create myapp
    Creating myapp... OK
```
### Getting information about an App
```bash
    $ convox apps info myapp
    Name    myapp
    Status  running
    Locked  false
    Release RABCDEFGHI
    Router  router.0a1b2c3d4e5f.convox.cloud
```
### Listing Apps
```bash
    $ convox apps
    APP    STATUS   RELEASE
    myapp  running  RABCDEFGHI
```
### Deleting an App
```bash
    $ convox apps delete myapp
    Deleting myapp... OK
```
### Getting logs for an App
```bash
    $ convox logs -a myapp
    2026-01-15T14:30:00 service/web/web-zyxwv Starting myapp on port 5000
```
### Cancelling a deployment that is in progress
```bash
    $ convox apps cancel myapp
    Cancelling deployment of myapp... OK
```
### Preventing accidental deletion of an App
```bash
    $ convox apps lock myapp
    Locking myapp... OK

    $ convox apps unlock myapp
    Unlocking myapp... OK
```
### Exporting an App
```bash
    $ convox apps export myapp -f /tmp/myapp.tgz
    Exporting app myapp... OK
    Exporting env... OK
    Exporting build BABCDEFGHI... OK
    Exporting resource database... OK
    Packaging export... OK
```
### Importing an App
```bash
    $ convox apps import myapp2 -f /tmp/myapp.tgz
    Creating app myapp2... OK
    Importing build... OK, RIHGFEDCBA
    Importing env... OK, RJIHGFEDCB
    Promoting RJIHGFEDCB... OK
    Importing resource database... OK
```