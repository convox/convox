---
title: "Scaling"
draft: false
slug: Scaling
url: /deployment/scaling
---
# Scaling

Convox allows you to easily scale any [Service](/reference/primitives/app/service) on the following dimensions:

- Horizontal concurrency (number of [Processes](/reference/primitives/app/process))
- CPU allocation (in CPU units where 1000 units is one full CPU)
- Memory allocation (in MB)

## Initial Defaults

You can specify the scale for any [Service](/reference/primitives/app/service) in your [convox.yml](/configuration/convox-yml)
```html
    services:
      web:
        scale:
          count: 2
          cpu: 250
          memory: 512
```
> If you specify a static `count` it will only be used on first deploy. Subsequent changes must be made using the `convox` CLI.

## Manual Scaling

### Determine Current Scale
```html
    $ convox scale
    NAME  DESIRED  RUNNING  CPU  MEMORY
    web   2        2        250  512
```
### Scaling Count Horizontally
```html
    $ convox scale web --count=3
    Scaling web...
    2020-01-01T00:00:00Z system/k8s/web Scaled up replica set web-65f45567d to 2
    2020-01-01T00:00:00Z system/k8s/web-65f45567d Created pod: web-65f45567d-c7sdw
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Successfully assigned dev-convox/web-65f45567d-c7sdw to node
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Container image "registry.dev.convox/convox:web.BABCDEFGHI" already present on machine
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Created container main
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Started container main
    OK
```
> Changes to `cpu` or `memory` should be done in your `convox.yml`, and a new release of your app deployed.

## Autoscaling

To use autoscaling you must specify a range for allowable [Process](/reference/primitives/app/process) count and
target values for CPU and Memory utilization (in percent):
```html
    service:
      web:
        scale:
          count: 1-10
          targets:
            cpu: 70
            memory: 90
```
The number of [Processes](/reference/primitives/app/process) will be continually adjusted to maintain your target metrics.

You must consider that the targets for CPU and Memory use the service replicas limits to calculate the utilization percentage. So if you set the target for CPU as `70` and have two replicas, it will trigger the auto-scale only if the utilization percentage sum divided by the replica's count is bigger than 70%. The desired replicas will be calculated to satisfy the percentage. Being the `currentMetricValue` computed by taking the average of the given metric across all service replicas.

```html
desiredReplicas = ceil[currentReplicas * ( currentMetricValue / desiredMetricValue )]
```
