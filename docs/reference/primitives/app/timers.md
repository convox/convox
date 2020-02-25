# Timer

A Timer spawns a [Process](process.md) on a schedule. The schedule is defined using `cron` syntax. A timer is the equivalent of issuing a [convox run](../../cli/run.md) command but on a defined schedule. An application can have multiple timers defined.

## Definition

A Timer is defined in [`convox.yml`](../../../configuration/convox.yml.md).

    timers:
      cleanup:
        schedule: "0 3 * * ? *"
        command: bin/cleanup
        service: worker

### Attributes

| Name       | Required | Description                                                           |
| ---------- | -------- | --------------------------------------------------------------------- |
| `schedule` | **yes**  | A cron formatted schedule for spawning the process. All times are UTC |
| `command`  | **yes**  | The command to run that spawns the process                            |
| `service`  | **yes**  | The name of the service that the command will run against             |

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

Two services, `web` is normally running, `timers` is not (scaled to 0). The `cleanup` timer will spawn a new process using the configuration of `timers` once per minute, run the command `bin/cleanup` inside it, and terminate on completion.

    services:
      web:
        build: .
        command: bin/webserver
      timers:
        build: ./timers
        scale: 0
    timers:
      cleanup:
        command: bin/cleanup
        schedule: "*/1 * * * ?"
        service: timers

#### Existing Service

One service `web` is normally running. The `cleanup` timer will spawn a new process using the configuration of `web` one per minute, run the command `bin/cleanup` inside it, and terminate on completion.

    services:
      web:
        build: .
        command: bin/webserver
    timers:
      cleanup:
        command: bin/cleanup
        schedule: "*/1 * * * ?"
        service: web
