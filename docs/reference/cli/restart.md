---
title: "restart"
slug: restart
url: /reference/cli/restart
---
# restart

The `convox restart` command performs a rolling restart of all services in an app. Each service is restarted sequentially to help maintain availability during the process.

## restart

Restart an app

### Usage
```bash
    convox restart
```
### Examples
```bash
    $ convox restart
    Restarting app... OK
```

## See Also

- [services](/reference/cli/services) for restarting individual services
- [deploy](/reference/cli/deploy) for deploying new code
- [Rolling Updates](/deployment/rolling-updates) for how restarts maintain availability