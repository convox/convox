---
order: 1
---

# convox.yml

`convox.yml` is a manifest used to describe your application and all of its infrastructure needs.

    environment:
      - COMPANY=Convox
      - DOCS_URL
    resources:
      database:
        type: postgres
        options:
          storage: 200
      queue:
        type: redis
    services:
      api:
        build: .
        command: bin/api
        environment:
          - ENCRYPTION_KEY
        health: /check
        internal: true
        port: 3000
        resources:
          - database
          - queue
        test: make test
      web:
        build: .
        command: bin/web
        environment:
          - SESSION_SECRET
        port: 3000
      worker:
        build: ./worker
        command: bin/worker
        environment:
          - ENCRYPTION_KEY
        resources:
          - database
          - queue
      metrics:
        agent: true
        image: awesome/metrics
    timers:
      cleanup:
        schedule: "0 3 * * ? *"
        command: bin/cleanup
        service: worker

## environment

The top-level `environment` section defines [Environment Variables](environment.md) that are available to every
[Service](../reference/primitives/app/service.md).

    environment:
      - COMPANY=Convox  # has a default value of "Convox"
      - DOCS_URL        # must be set before deployment
  
See [Environment Variables](environment.md) for configuration options.

## resources

The `resources` section defines network-accessible [Resources](../reference/primitives/app/resource.md)
such as databases that can be made available to [Services](../references/primitives/app/service.md).

    resources:
      database:
        type: postgres
        options:
          storage: 200

See [Resource](../reference/primitives/app/resource.md) for configuration options.

## services

The `services` section horizontally-scalable [Services](../reference/primitives/app/service.md)
that can be optionally placed behind a load balancer.

    services:
      api:
        build: .
        command: bin/api
        environment:
          - ENCRYPTION_KEY
        health: /check
        internal: true
        port: 3000
        resources:
          - database
          - queue
        test: make test

See [Service](../reference/primitives/app/service.md) for configuration options.

## timers

The `timers` section defines [Processes](../reference/primitives/app/process.md)
that run periodically on a set interval.

    timers:
      cleanup:
        schedule: "0 3 * * ? *"
        command: bin/cleanup
        service: worker

See [Timer](../reference/primitives/app/timer.md) for configuration options.
