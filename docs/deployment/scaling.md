# Scaling

Convox allows you to easily scale any [Service](../reference/primitives/app/service.md) on the following dimensions:

- Horizontal concurrency (number of [Processes](../reference/primitives/app/process.md))
- CPU allocation (in CPU units where 1024 units is one full CPU)
- Memory allocation (in MB)
 
## Initial Defaults

You can specify the scale for any [Service](../reference/primitives/app/service.md) in your [convox.yml](../configuration/convox-yml.md)

    services:
      web:
        scale:
          count: 2
          cpu: 256
          memory: 512

> If you specify a static `count` it will only be used on first deploy. Subsequent changes must be made using the `convox` CLI.

## Manual Scaling

### Determine Current Scale

    $ convox scale
    NAME  DESIRED  RUNNING  CPU  MEMORY
    web   2        2        256  512

### Scaling Horizontally

    $ convox scale web --count=3
    Scaling web...
    2020-01-01T00:00:00Z system/k8s/web Scaled up replica set web-65f45567d to 2
    2020-01-01T00:00:00Z system/k8s/web-65f45567d Created pod: web-65f45567d-c7sdw
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Successfully assigned dev-convox/web-65f45567d-c7sdw to node
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Container image "registry.dev.convox/convox:web.BABCDEFGHI" already present on machine
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Created container main
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Started container main
    OK

## Autoscaling

To use autoscaling you must specify a range for allowable [Process](../reference/primitives/app/process.md) count and
target values for CPU and Memory utilization (in percent):

    service:
      web:
        scale:
          count: 1-10
          targets:
            cpu: 70
            memory: 90

The number of [Processes](../reference/primitives/app/process.md) will be continually adjusted to maintain
your target metrics.