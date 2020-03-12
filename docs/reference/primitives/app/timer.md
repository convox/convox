# Timer

A Timer spawns a [Process](process.md) on a schedule that is defined using [cron syntax](https://www.freebsd.org/cgi/man.cgi?query=crontab&sektion=5).

## Definition

A Timer is defined in [`convox.yml`](../../../configuration/convox.yml.md).

    services:
      worker:
        build: ./worker
    timers:
      cleanup:
        command: bin/cleanup
        schedule: "0 3 * * *"
        service: worker

### Attributes

| Name       | Required | Description                                                                                |
| ---------- | -------- | ------------------------------------------------------------------------------------------ |
| `command`  | **yes**  | The command to execute once the [Process](process.md) starts                               |
| `schedule` | **yes**  | A cron formatted schedule for spawning the [Process](process.md). All times are UTC        |
| `service`  | **yes**  | The name of the [Service](service.md) that will be used to spawn the [Process](process.md) |

### Cron Expression Format

Cron expressions use the following format. All times are UTC.

```
.----------------- minute (0 - 59)
|  .-------------- hour (0 - 23)
|  |  .----------- day-of-month (1 - 31)
|  |  |  .-------- month (1 - 12) OR JAN,FEB,MAR,APR ...
|  |  |  |  .----- day-of-week (0 - 6) OR SUN,MON,TUE,WED,THU,FRI,SAT
|  |  |  |  |
*  *  *  *  *
```

### Using a Template Service

Timers can run against any [Service](service.md), even one that is scaled to zero. You can use this to create a
template [Service](service.md) for your Timers.

    services:
      web:
        build: .
        command: bin/web
        port: 5000
      jobs:
        build: ./jobs
        scale: 
          count: 0
    timers:
      cleanup:
        command: bin/cleanup
        schedule: "*/2 * * * *"
        service: jobs

On this [App](..) the `jobs` [Service](service.md) is scaled to zero and not running any durable
[Processes](process.md).

The `cleanup` Timer will spawn a [Process](process.md) of the `jobs` [Service](service.md) to run
`bin/cleanup` once every two minutes.
