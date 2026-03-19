---
title: "convox.yml"
slug: convox-yml
url: /configuration/convox-yml
---

# convox.yml

`convox.yml` is a manifest used to describe your application and all of its infrastructure needs.
```yaml
    environment:
      - COMPANY=Convox
      - DOCS_URL
    labels:
      team: platform
    configs:
      - id: app-config
    appSettings:
      awsLogs:
        cwRetention: 31
        disableRetention: false
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
        schedule: "0 3 * * *"
        command: bin/cleanup
        service: worker
```
## environment

The top-level `environment` section defines [Environment Variables](/configuration/environment) that are available to every
[Service](/reference/primitives/app/service).
```yaml
    environment:
      - COMPANY=Convox  # has a default value of "Convox"
      - DOCS_URL        # must be set before deployment
```
See [Environment Variables](/configuration/environment) for configuration options.

## app settings

The `appSettings` section defines settings that apply exclusively to a particular app within the rack. These settings are independent of the global rack-level parameters and provide a mechanism for tailoring configuration to individual applications.

```yaml
    appSettings:
      awsLogs:
        cwRetention: 31
        disableRetention: false
```
See [App Settings](/configuration/app-settings) for configuration options.

## resources

The `resources` section defines network-accessible [Resources](/reference/primitives/app/resource)
such as databases that can be made available to [Services](/reference/primitives/app/service).
```yaml
    resources:
      database:
        type: postgres
        options:
          storage: 200
```
See [Resource](/reference/primitives/app/resource) for configuration options.

## services

The `services` section defines horizontally-scalable [Services](/reference/primitives/app/service)
that can be optionally placed behind a load balancer.
```yaml
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
See [Service](/reference/primitives/app/service) for configuration options. A Service can also be declared as an [Agent](/configuration/agents) to run one process on every node.

## labels

The top-level `labels` section defines labels applied to all [Services](/reference/primitives/app/service) in the app. Service-level labels override top-level labels with the same key.
```yaml
    labels:
      team: platform
      cost-center: engineering
```
The following label keys are reserved and cannot be used: `system`, `rack`, `app`, `name`, `service`, `release`, `type`.

## configs

The `configs` section defines named configuration objects that can be mounted into service containers as files.
```yaml
    configs:
      - id: app-config
```
See [Config Mounts](/configuration/config-mounts) for configuration options.

## timers

The `timers` section defines [Processes](/reference/primitives/app/process)
that run periodically on a set interval.
```yaml
    timers:
      cleanup:
        schedule: "0 3 * * *"
        command: bin/cleanup
        service: worker
```
See [Timer](/reference/primitives/app/timer) for configuration options.

## balancers

The `balancers` section defines custom TCP/UDP load balancers for [Services](/reference/primitives/app/service) that need to expose arbitrary ports.
```yaml
    balancers:
      custom:
        service: web
        ports:
          5000: 5001
```
See [Balancer](/reference/primitives/app/balancer) and [Load Balancers](/configuration/load-balancers#custom-load-balancers) for configuration options.

## See Also

- [Environment Variables](/configuration/environment) for configuring app settings
- [Health Checks](/configuration/health-checks) for configuring service health checks
- [Scaling](/configuration/scaling) for controlling service scale and autoscaling
- [Config Mounts](/configuration/config-mounts) for mounting configuration files into containers
