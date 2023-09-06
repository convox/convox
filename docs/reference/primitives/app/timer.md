---
title: "Timer"
draft: false
slug: Timer
url: /reference/primitives/app/timer
---
# Timer

A Timer spawns a [Process](/reference/primitives/app/process) on a schedule that is defined using [cron syntax](https://crontab.guru).

## Definition

A Timer is defined in [`convox.yml`](/configuration/convox-yml).
```html
    services:
      worker:
        build: ./worker
    timers:
      cleanup:
        annotations:
          - test.annotation.org/value=foobar
        command: bin/cleanup
        schedule: "0 3 * * *"
        service: worker
```
### Attributes

| Name       | Required | Description                                                                                |
| ---------- | -------- | ------------------------------------------------------------------------------------------ |
| **annotations** | **no**     | A list of annotation keys and values to populate the metadata for the deployed pods and their serviceaccounts. Supported version >= 3.13.5|
| **command**      | **yes**  | The command to execute once the [Process](/reference/primitives/app/process) starts                               |
| **schedule**     | **yes**  | A cron formatted schedule for spawning the [Process](/reference/primitives/app/process). All times are UTC        |
| **service**      | **yes**  | The name of the [Service](/reference/primitives/app/service) that will be used to spawn the [Process](/reference/primitives/app/process) |
| **concurrency**  | **no**   | It specifies how to treat concurrent executions of a job that is created by this cron job. The default value for this field is `Allow` if is not defined. Check this [doc](https://kubernetes.io/docs/tasks/job/automated-tasks-with-cron-jobs/#concurrency-policy) for more info. |

### Cron Expression Format

Cron expressions use the following format. All times are UTC.

```html
.----------------- minute (0 - 59)
|  .-------------- hour (0 - 23)
|  |  .----------- day-of-month (1 - 31)
|  |  |  .-------- month (1 - 12) OR JAN,FEB,MAR,APR ...
|  |  |  |  .----- day-of-week (0 - 6) OR SUN,MON,TUE,WED,THU,FRI,SAT
|  |  |  |  |
*  *  *  *  *
```

Please notice that the smallest unit of time here is **minute**.

### Using a Template Service

Timers can run against any [Service](/reference/primitives/app/service), even one that is scaled to zero. You can use this to create a
template [Service](/reference/primitives/app/service) for your Timers.
```html
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
        concurrency: forbid
```
On this [App](..) the `jobs` [Service](/reference/primitives/app/service) is scaled to zero and not running any durable
[Processes](/reference/primitives/app/process).

The `cleanup` Timer will spawn a [Process](/reference/primitives/app/process) of the `jobs` [Service](/reference/primitives/app/service) to run
`bin/cleanup` once every two minutes.
