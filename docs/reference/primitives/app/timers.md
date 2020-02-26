# Timer

A Timer spawns a [Process](process.md) on a schedule. The schedule is defined using `cron` syntax. A Timer is the equivalent of issuing a [convox run](../../cli/run.md) command but on a defined schedule. An [App](../app.md) can have multiple Timers defined.

## Definition

A Timer is defined in [`convox.yml`](../../../configuration/convox.yml.md).

    timers:
      cleanup:
        schedule: "0 3 * * ? *"
        command: bin/cleanup
        service: worker

### Attributes

| Name       | Required | Description                                                                                                                                                     |
| ---------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `schedule` | **yes**  | A cron formatted schedule for spawning the [Process](process.md). All times are UTC                                                                             |
| `command`  | **yes**  | The command to execute once the [Process](process.md) starts                                                                                                    |
| `service`  | **yes**  | The name of the [Service](service.md) as defined in your [`convox.yml`](../../../configuration/convox.yml.md) whose configuration is used to launch the process |

### Cron expression format

Cron expressions use the following format. All times are UTC.

```
.----------------- minute (0 - 59)
|  .-------------- hour (0 - 23)
|  |  .----------- day-of-month (1 - 31)
|  |  |  .-------- month (1 - 12) OR JAN,FEB,MAR,APR ...
|  |  |  |  .----- day-of-week (1 - 7) OR SUN,MON,TUE,WED,THU,FRI,SAT
|  |  |  |  |  .-- year (1970 - 2199)
|  |  |  |  |  |
*  *  *  *  *  *
```

### Examples

#### Dedicated Service

Two [Services](service.md), `web` is normally running, `worker` is not (scaled to 0). The `cleanup` timer will spawn a new [Process](process.md) using the configuration of `worker` once per minute, run the command `bin/cleanup` inside it, and terminate on completion.

    services:
      web:
        build: .
        command: bin/webserver
      worker:
        build: ./worker
        scale: 
          count: 0
    timers:
      cleanup:
        command: bin/cleanup
        schedule: "*/1 * * * ?"
        service: worker

#### Existing Service

One [Service](service.md) `web` is normally running. The `cleanup` timer will spawn a new [Process](process.md) using the configuration of `web` one per minute, run the command `bin/cleanup` inside it, and terminate on completion.

    services:
      web:
        build: .
        command: bin/webserver
    timers:
      cleanup:
        command: bin/cleanup
        schedule: "*/1 * * * ?"
        service: web
