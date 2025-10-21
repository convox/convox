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
| **parallelCount** | **no**  | The number of parallel replicas to run for each timer execution. Defaults to 1. Each replica receives a unique `TIMER_INDEX` environment variable (0-based). Supported version >= 3.22.4 |

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

## Parallel Timer Execution

Timers support running multiple parallel replicas for increased throughput and distributed processing of scheduled tasks. This is particularly useful for high-volume batch processing, data pipelines, or any scheduled workload that benefits from horizontal scaling.

### Configuring Parallel Execution

To enable parallel execution, use the `parallelCount` attribute:

```html
    services:
      worker:
        build: ./worker
    timers:
      data-processor:
        command: bin/process-data
        schedule: "0 * * * *"
        service: worker
        parallelCount: 5
```

In this example, the `data-processor` timer will spawn **5 parallel replicas** every hour, each running the `bin/process-data` command simultaneously.

### Using TIMER_INDEX for Work Distribution

Each parallel replica receives a unique `TIMER_INDEX` environment variable (starting from 0) that can be used to partition work across instances:

```html
    timers:
      cleanup:
        command: ./cleanup.sh
        schedule: "*/10 * * * *"
        service: worker
        parallelCount: 3
```

Each replica receives:
- First container: `TIMER_INDEX=0`
- Second container: `TIMER_INDEX=1`  
- Third container: `TIMER_INDEX=2`

### Example: Partitioned Data Processing

Here's an example of using `TIMER_INDEX` to partition work across timer replicas:

```bash
#!/bin/bash
# cleanup.sh - Partition cleanup work across replicas

TOTAL_REPLICAS=3
PARTITION=$TIMER_INDEX

echo "Processing partition $PARTITION of $TOTAL_REPLICAS"

# Process different data segments based on replica index
case $TIMER_INDEX in
  0)
    echo "Processing records where id % 3 = 0"
    psql $DATABASE_URL -c "DELETE FROM logs WHERE created_at < NOW() - INTERVAL '30 days' AND id % 3 = 0"
    ;;
  1)
    echo "Processing records where id % 3 = 1"
    psql $DATABASE_URL -c "DELETE FROM logs WHERE created_at < NOW() - INTERVAL '30 days' AND id % 3 = 1"
    ;;
  2)
    echo "Processing records where id % 3 = 2"
    psql $DATABASE_URL -c "DELETE FROM logs WHERE created_at < NOW() - INTERVAL '30 days' AND id % 3 = 2"
    ;;
esac
```

### Use Cases for Parallel Timers

Parallel timer execution is valuable for:

- **Data Processing Pipelines**: Process large datasets by partitioning work across replicas
- **Cleanup Operations**: Parallelize deletion or archival tasks across different data segments
- **Report Generation**: Generate multiple reports simultaneously for different regions or departments
- **Import/Export Jobs**: Handle multiple data sources or destinations concurrently
- **Monitoring Tasks**: Check different service endpoints or regions in parallel

### Important Considerations

When using parallel timers, keep in mind:

1. **Concurrency Safety**: Ensure your timer logic can handle concurrent execution without conflicts
2. **Database Locking**: Be aware of potential database locks or race conditions
3. **Resource Limits**: Each replica consumes resources - ensure your cluster has sufficient capacity
4. **Idempotency**: Design timer operations to be idempotent where possible
5. **Concurrency Policy**: The `concurrency` attribute still applies to the timer as a whole, controlling whether new timer executions can start while previous ones are running

### Complete Example with Parallel Execution

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
      hourly-import:
        command: bin/import-data
        schedule: "0 * * * *"
        service: jobs
        parallelCount: 4
        concurrency: forbid
      daily-cleanup:
        command: bin/cleanup
        schedule: "0 2 * * *"
        service: jobs
        parallelCount: 10
        annotations:
          - monitoring.example.com/alert=true
```

In this configuration:
- `hourly-import` runs 4 parallel import processes every hour, with `concurrency: forbid` ensuring no overlapping executions
- `daily-cleanup` runs 10 parallel cleanup processes at 2 AM UTC daily
- Both timers use the `jobs` service template which is scaled to zero when not in use