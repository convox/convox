---
title: "Cloud Databases"
draft: false
slug: databases
url: /cloud/databases
---

# Cloud Databases

Convox Cloud Databases provide fully managed RDS instances without a subscription. Pay only for the hours you use. Every tier includes 7-day automated backups and optional Multi-AZ failover, so you can match your database to your workload from development environments to high-traffic production.

## Supported Database Types

Convox Cloud supports three database engines:

- [PostgreSQL](/cloud/databases/postgres) - Versions 14.x through 18.x
- [MySQL](/cloud/databases/mysql) - Versions 8.0.x and 8.4.x
- [MariaDB](/cloud/databases/mariadb) - Versions 10.6.x through 11.8.x

## Definition

Cloud Databases are defined in your `convox.yml` with `provider: aws`:

```yaml
resources:
  database:
    type: postgres
    provider: aws
    options:
      class: small
      version: 17.5
      
services:
  web:
    build: .
    port: 3000
    resources:
      - database
```

### Attributes

| Name | Required | Description |
|------|----------|-------------|
| **type** | yes | Database engine: `postgres`, `mysql`, or `mariadb` |
| **provider** | yes | Must be `aws` for Cloud Databases |
| **options.class** | no | Database size: `dev`, `small`, `medium`, or `large` (default: `dev`) |
| **options.version** | no | Database engine version (see supported versions below) |
| **options.durable** | no | Enable Multi-AZ failover for high availability (default: `false`) |

## Database Classes

| Class | vCPU | RAM | Storage | Use Case |
|-------|------|-----|---------|----------|
| **dev** | 1.0 | 1 GB | 20 GB | Development, testing, sandbox environments |
| **small** | 2.0 | 2 GB | 50 GB | Small production apps, light to moderate traffic |
| **medium** | 2.0 | 4 GB | 100 GB | Growing applications, higher connection limits |
| **large** | 2.0 | 8 GB | 250 GB | High-traffic production, enterprise workloads |

## Linking Resources

Linking a database to a [Service](/reference/primitives/app/service) injects environment variables into the service's processes:

```yaml
resources:
  main:
    type: postgres
    provider: aws
    options:
      class: small
      version: 17.5

services:
  web:
    resources:
      - main
```

The following environment variables are automatically set:

```
MAIN_URL=postgres://username:password@host.name:port/database
MAIN_USER=username
MAIN_PASS=password
MAIN_HOST=host.name
MAIN_PORT=port
MAIN_NAME=database
```

## Multi-AZ (Durable) Configuration

Enable Multi-AZ deployment for automatic failover and high availability:

```yaml
resources:
  database:
    type: postgres
    provider: aws
    options:
      class: medium
      version: 17.5
      durable: true
```

When `durable: true` is set:

- A standby replica is created in a different availability zone
- Automatic failover occurs if the primary instance fails
- Monthly cost is doubled (two instances running)

## Example Configurations

### Development Environment

```yaml
resources:
  database:
    type: postgres
    provider: aws
    options:
      class: dev
      version: 17.5

services:
  web:
    build: .
    port: 3000
    resources:
      - database
```

### Production with Multiple Databases

```yaml
resources:
  postgres-main:
    type: postgres
    provider: aws
    options:
      class: medium
      version: 17.5
      durable: true
      
  mysql-legacy:
    type: mysql
    provider: aws
    options:
      class: small
      version: 8.4.6
      
  mariadb-analytics:
    type: mariadb
    provider: aws
    options:
      class: large
      version: 11.4.8

services:
  web:
    build: .
    port: 3000
    resources:
      - postgres-main
      - mysql-legacy
      
  analytics:
    build: ./analytics
    resources:
      - mariadb-analytics
```

## Command Line Interface

### Listing Resources

```bash
$ convox cloud resources -a myapp -i my-machine
NAME           TYPE      URL
postgres-main  postgres  postgres://username:password@host.name:5432/database
```

### Getting Resource Information

```bash
$ convox cloud resources info postgres-main -a myapp -i my-machine
Name  postgres-main
Type  postgres
URL   postgres://username:password@host.name:5432/database
```

### Launching a Console

```bash
$ convox cloud resources console postgres-main -a myapp -i my-machine
psql (17.5)
Type "help" for help.
database=#
```

### Starting a Proxy

```bash
$ convox cloud resources proxy postgres-main -a myapp -i my-machine
Proxying localhost:5432 to host.name:5432
```

### Exporting Data

```bash
$ convox cloud resources export postgres-main -a myapp -i my-machine --file backup.sql
Exporting data from postgres-main... OK
```

### Importing Data

```bash
$ convox cloud resources import postgres-main -a myapp -i my-machine --file backup.sql
Importing data to postgres-main... OK
```

## Next Steps

- [PostgreSQL](/cloud/databases/postgres) - PostgreSQL-specific configuration and versions
- [MySQL](/cloud/databases/mysql) - MySQL-specific configuration and versions
- [MariaDB](/cloud/databases/mariadb) - MariaDB-specific configuration and versions
- [Sizing and Pricing](/cloud/databases/sizing-and-pricing) - Detailed pricing information