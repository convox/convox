---
title: "Resource Management (Cloud Databases)"
description: "The convox cloud resources commands list, inspect, open a console, proxy, export, and import data for managed Cloud Databases defined in convox.yml."
slug: resources
url: /cloud/cli-reference/resources
---

# Resource Management (Cloud Databases)

Cloud Databases are defined in your `convox.yml` using a managed resource type. The CLI provides commands to interact with these managed databases.

### resources

List resources for an application.

```bash
$ convox cloud resources -a <app> -i <machine>
```

**Options:**
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud resources -a myapp -i production
NAME      TYPE      URL
database  postgres  postgres://user:pass@host:5432/db
```

### resources console

Open a console for a resource.

```bash
$ convox cloud resources console <resource> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud resources console database -a myapp -i production
psql (17.5)
Type "help" for help.
db=#
```

### resources export

Export data from a resource.

```bash
$ convox cloud resources export <resource> -a <app> -i <machine>
```

**Options:**
- `--file`: Output file path

**Example:**
```bash
$ convox cloud resources export database -a myapp -i production --file backup.sql
Exporting data from database... OK
```

### resources import

Import data to a resource.

```bash
$ convox cloud resources import <resource> -a <app> -i <machine>
```

**Options:**
- `--file`: Input file path

**Example:**
```bash
$ convox cloud resources import database -a myapp -i staging --file backup.sql
Importing data to database... OK
```

### resources info

Get information about a resource.

```bash
$ convox cloud resources info <resource> -a <app> -i <machine>
```

### resources proxy

Proxy a local port to a resource.

```bash
$ convox cloud resources proxy <resource> -a <app> -i <machine>
```

**Options:**
- `--port`: Local port
- `--tls`: Enable TLS

**Example:**
```bash
$ convox cloud resources proxy database -a myapp -i production --port 5433
Proxying localhost:5433 to database.internal:5432
```

### resources url

Get the connection URL for a resource.

```bash
$ convox cloud resources url <resource> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud resources url database -a myapp -i production
postgres://user:pass@host:5432/db
```
