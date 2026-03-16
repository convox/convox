---
title: "Logging"
slug: logging
url: /configuration/logging
---
# Logging

Convox automatically captures and stores logs for:

* All output to `stdout` or `stderr` made by any running [Process](/reference/primitives/app/process)
* State changes triggered by deployments
* Health check failures

## Command Line Interface

### Viewing Logs for an App

#### Simple
```bash
    $ convox logs -a myapp
    2020-01-01T00:00:00Z service/web/012345689 starting on port 3000
    2020-01-01T00:00:01Z service/web/012345689 GET / 200
    2020-01-01T00:00:02Z service/web/012345689 GET /other 404
```
#### Advanced
```bash
    $ convox logs -a myapp --since 20m --no-follow
    2020-01-01T00:00:00Z service/web/012345689 starting on port 3000
    2020-01-01T00:00:01Z service/web/012345689 GET / 200
    2020-01-01T00:00:02Z service/web/012345689 GET /other 404
```

## See Also

- [Monitoring and Alerting](/configuration/monitoring) for setting up monitoring
- [Datadog Integration](/integrations/monitoring/datadog) for forwarding logs to Datadog
