---
title: "Postgis"
slug: postgis
url: /reference/primitives/app/resource/postgis
---
# Postgis

## Definition

A Postgis Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service). PostGIS extends PostgreSQL with spatial and geographic data types, functions, and indexes.

```yaml
resources:
  geodatabase:
    type: postgis
services:
  web:
    resources:
      - geodatabase
```

## Options

A Postgis Resource can have the following options configured for it (default values are shown):

```yaml
resources:
  geodatabase:
    type: postgis
    options:
      version: 10-3.2
      storage: 10
```

| Attribute   | Type   | Default  | Description                                  |
| ----------- | ------ | -------- | -------------------------------------------- |
| **version** | string | `10-3.2` | The PostGIS image tag (postgres version-postgis version) |
| **storage** | int    | `10`     | The amount of storage (in GB) to allocate    |

> For AWS RDS managed PostGIS databases, use the `rds-postgres` resource type with PostGIS extensions enabled at the RDS level. See the [PostgreSQL](/reference/primitives/app/resource/postgres#aws-rds-managed-postgres-resources) resource documentation for RDS configuration options.

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
psql (10.20, server 10.20 (Debian 10.20-1.pgdg90+1))
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
