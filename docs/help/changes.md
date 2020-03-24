# Changes

This document outlines the changes from Version 2 (date-based version) Racks to Version 3.x Racks.

## Racks

### Versioning

- "Version 2" Racks have versions like `20200302115619` (reverse date format, start with a 2)
- "Version 3" Racks have versions like `3.0.8` (Semantic versioning, start with a 3)

### Underling Infrastructure

- v2 Racks run on ECS (AWS) only
- v3 Racks run on:
  - EKS (AWS),
  - GKE (Google Cloud),
  - AKS (Azure),
  - Managed Kubernetes (Digital Ocean)
  - so far...

### Development status

- v2 Racks are supported for Pro & Enterprise Customers
- v3 Racks are under active development

### Console and CLI Support

- v2 Racks have Console based install and CLI based install, management and uninstall
- v3 Racks have Console based install, management and uninstall and CLI based install, management and uninstall

### App Generation Support

- v2 Racks run Gen1 and Gen2 apps
- v3 Racks run Generation 2 Apps

## Apps

### Agent Ports

Agent ports are now defined at the service level instead of underneath the `agent:` block:

#### Version 2

    services:
      datadog:
        agent:
          ports:
            - 8125/udp
            - 8126/tcp

#### Version 3

    services:
      datadog:
        agent: true
        ports:
          - 8125/udp
          - 8126/tcp

### Sticky Sessions

App services are no longer sticky by default. Sticky sessions can be enabled in `convox.yml`:

    services
      web:
        sticky: true

### Timer Syntax

Timers no longer follow the AWS scheduled events syntax where you must have a `?` in either day-of-week or day-of-month column. 

Timers now follow the standard [cron syntax](https://www.freebsd.org/cgi/man.cgi?query=crontab&sektion=5)

As an example a Timer that runs every hour has changed as follows:

#### Version 2

    timers:
      hourlyjob:
        schedule: 0 * * ? *

#### Version 3

    timers:
      hourlyjob:
        schedule: 0 * * * *


You can read more in the [Timer](../reference/primitives/app/timer.md) documentation section