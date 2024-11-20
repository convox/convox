---
title: "logs"
draft: false
slug: logs
url: /reference/cli/logs
---
# Logs

## Logs

Get logs for an app

### Usage
```html
    convox logs
```
### Examples
```html
    $ convox logs
    2020-02-05T12:47:41Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=2bdd60aaf431 route=root at=end state=success elapsed=0.065
    2020-02-05T12:47:41Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=2bdd60aaf431 route=root at=start method="GET" path="/" elapsed=0.029
    2020-02-05T12:47:43Z service/web/a81ba08c-6dbe-48a4-88e6-da5f940156ae ns=template id=57c9464c88f6 route=root at=end state=success elapsed=0.070
    2020-02-05T12:47:43Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=f5b0fcdd6f63 route=root at=start method="GET" path="/" elapsed=0.038
    ....

    $ convox logs --filter 2bdd60aaf431 --since 24h
    2020-02-05T12:47:41Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=2bdd60aaf431 route=root at=end state=success elapsed=0.065
    2020-02-05T12:47:41Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=2bdd60aaf431 route=root at=start method="GET" path="/" elapsed=0.029
```

### Options

- `--app` - Specify application for logging 
- `--rack` - Specify rack for logging 
- `--filter` - Filter for a specific string within the logs. This is not applicable for service specific logging.
- `--since` - Set time frame for log query  
- `--no-follow` - Prints logs in terminal rather than opening a log stream
- `--service` or `-s` - Sepcify the name of the service
- `--tail` - Specify the number of lines to tail. This is only applicable on service specific logging.
