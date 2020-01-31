# Health Checks

Deploying a [Service](../reference/primitives/app/service.md) behind a load balancer requires a health
check to determine whether a given [Process](../reference/primitives/app/process.md) is ready to
handle requests.

Health checks must return a valid HTTP response code (200-399) within the configured `timeout`.

[Processes](../reference/primitives/app/process.md) that fail two health checks in a row are assumed
dead and will be terminated and replaced.

## Definition

### Simple

    services:
      web:
        health: /check

> Specifying `health` as a string will set the `path` and leave the other options as defaults.

### Advanced

```
services:
  web:
    health:
      grace: 5
      interval: 5
      path: /check
      timeout: 3
```

| Attribute  | Default | Description                                                                                                                          |
| ---------- | ------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `grace`    | 5       | The amount of time in seconds to wait for a [Process](../reference/primitives/app/process.md) to boot before beginning health checks |
| `interval` | 5       | The number of seconds between health checks                                                                                          |
| `path`     | /       | The HTTP endpoint that will be requested                                                                                             |
| `timeout`  | 4       | The number of seconds to wait for a valid response                                                                                   |