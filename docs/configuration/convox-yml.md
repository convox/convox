---
title: "convox.yml"
draft: false
slug: convox.yml
url: /configuration/convox-yml
---

# convox.yml

`convox.yml` is a manifest used to describe your application and all of its infrastructure needs.
```html
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
        annotations:
          - test.annotation.org/value=foobar
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
        termination:
          grace: 45
        test: make test
        timeout: 120
        deployment:
          minimum: 50
          maximum: 200
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
        schedule: "0 3 * * * *"
        command: bin/cleanup
        service: worker
```
## environment

The top-level `environment` section defines [Environment Variables](/configuration/environment) that are available to every
[Service](/reference/primitives/app/service).
```html
    environment:
      - COMPANY=Convox  # has a default value of "Convox"
      - DOCS_URL        # must be set before deployment
```
See [Environment Variables](/configuration/environment) for configuration options.

## resources

The `resources` section defines network-accessible [Resources](/reference/primitives/app/resource)
such as databases that can be made available to [Services](/reference/primitives/app/service).
```html
    resources:
      database:
        type: postgres
        options:
          storage: 200
```
See [Resource](/reference/primitives/app/resource) for configuration options.

## services

The `services` section horizontally-scalable [Services](/reference/primitives/app/service)
that can be optionally placed behind a load balancer.
```html
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
```
See [Service](/reference/primitives/app/service) for configuration options.

## timers

The `timers` section defines [Processes](/reference/primitives/app/process)
that run periodically on a set interval.
```html
    timers:
      cleanup:
        schedule: "0 3 * * * *"
        command: bin/cleanup
        service: worker
```
See [Timer](/reference/primitives/app/timer) for configuration options.
