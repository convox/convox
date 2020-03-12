# Changes

This document outlines the changes from Version 2 (date-based version) Racks to Version 3.x Racks.

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