---
title: "PostGIS"
slug: postgis
url: /reference/primitives/app/resource/postgis
---
# PostGIS

## Definition

A PostGIS Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service). PostGIS extends PostgreSQL with spatial and geographic data types, functions, and indexes.

```yaml
resources:
  geodatabase:
    type: postgis
services:
  web:
    resources:
      - geodatabase
```

## Containerized Options

Convox runs PostGIS as a container inside your Rack using the `postgis/postgis` Docker image. PostGIS is containerized-only -- there is no managed RDS equivalent.

For AWS RDS managed PostGIS, use the `rds-postgres` resource type with PostGIS extensions enabled at the RDS level. See the [PostgreSQL](/reference/primitives/app/resource/postgres#aws-rds-managed-postgres-resources) resource documentation for RDS configuration options.

```yaml
resources:
  geodatabase:
    type: postgis
    options:
      version: "16-3.4"
      storage: 20
```

| Attribute   | Type   | Default  | Description                                                       |
| ----------- | ------ | -------- | ----------------------------------------------------------------- |
| **version** | string | `10-3.2` | The PostGIS image tag (format: `postgres_version-postgis_version`) |
| **storage** | int    | `10`     | The amount of persistent storage (in GB)                          |

> Specify a recent version for production use. The default `10-3.2` is the template fallback; most deployments should set an explicit version such as `16-3.4` or `15-3.4`.

## Command Line Interface

### Listing Resources
```bash
$ convox resources -a myapp
NAME         TYPE     URL
geodatabase  postgis  postgres://username:password@host.name:port/geodatabase
```

### Getting Information about a Resource
```bash
$ convox resources info geodatabase -a myapp
Name  geodatabase
Type  postgis
URL   postgres://username:password@host.name:port/geodatabase
```

### Getting the URL for a Resource
```bash
$ convox resources url geodatabase -a myapp
postgres://username:password@host.name:port/geodatabase
```

### Launching a Console
```bash
$ convox resources console geodatabase -a myapp
psql (16.2, server 16.2 (Debian 16.2-1+b1))
Type "help" for help.
geodatabase=#
```

### Starting a Proxy to a Resource
```bash
$ convox resources proxy geodatabase -a myapp
Proxying localhost:5432 to host.name:port
```
> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.

### Exporting Data from a Resource
```bash
$ convox resources export geodatabase -f /tmp/db.sql
Exporting data from geodatabase... OK
```

### Importing Data to a Resource
```bash
$ convox resources import geodatabase -f /tmp/db.sql
Importing data to geodatabase... OK
```
