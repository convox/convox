---
title: "Service Management"
description: "The convox cloud service commands list services for an application, restart a service or the whole app, and scale a service's count, CPU, and memory."
slug: services
url: /cloud/cli-reference/services
---

# Service Management

### services

List services for an application.

```bash
$ convox cloud services -a <app> -i <machine>
```

**Options:**
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud services -a myapp -i production
SERVICE  DOMAIN                              PORTS
web      web.myapp.cloud.convox.com            443:3000
api      api.myapp.cloud.convox.com            443:8080
```

### services restart

Restart a service.

```bash
$ convox cloud services restart <service> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud services restart web -a myapp -i production
Restarting web... OK
```

### restart

Restart an entire application.

```bash
$ convox cloud restart -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud restart -a myapp -i production
Restarting app... OK
```

### scale

Scale a service.

```bash
$ convox cloud scale <service> -a <app> -i <machine>
```

**Options:**
- `--count`: Number of processes
- `--cpu`: CPU per process (millicores)
- `--memory`: Memory per process (MB)
- `--watch`: Watch scaling progress

**Example:**
```bash
$ convox cloud scale web --count 3 --cpu 500 --memory 1024 -a myapp -i production
Scaling web...
OK
```
