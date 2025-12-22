---
title: "MySQL"
draft: false
slug: mysql
url: /cloud/databases/mysql
---

# MySQL

Convox Cloud provides fully managed MySQL databases through AWS RDS.

## Definition

```yaml
resources:
  database:
    type: mysql
    provider: aws
    options:
      class: small
      version: 8.4.6

services:
  web:
    build: .
    port: 3000
    resources:
      - database
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| **class** | string | `dev` | Database size: `dev`, `small`, `medium`, or `large` |
| **version** | string | `8.4.6` | MySQL version |
| **durable** | boolean | `false` | Enable Multi-AZ failover |

## Supported Versions

| Version | Status |
|---------|--------|
| 8.4.7 | Available |
| 8.4.6 | Available |
| 8.4.5 | Available |
| 8.0.44 | Available |
| 8.0.43 | Available |
| 8.0.42 | Available |

## Database Classes

| Class | vCPU | RAM | Storage | Monthly Price |
|-------|------|-----|---------|---------------|
| **dev** | 1.0 | 1 GB | 20 GB | $19 |
| **small** | 2.0 | 2 GB | 50 GB | $39 |
| **medium** | 2.0 | 4 GB | 100 GB | $99 |
| **large** | 2.0 | 8 GB | 250 GB | $199 |

Enabling `durable: true` doubles the monthly cost.

## Features by Class

### Dev
- Development databases
- Staging environments
- Sandbox testing

### Small
- Production-ready performance
- Light to moderate traffic
- Optional Multi-AZ failover

### Medium
- Growing applications
- Higher connection limits
- Read replica support

### Large
- High-traffic production
- Maximum connection capacity
- Enterprise workloads

## All Classes Include

- 7-day automated backups
- Automatic minor version upgrades
- SSL/TLS encryption in transit
- Encryption at rest

## Examples

### Development Database

```yaml
resources:
  database:
    type: mysql
    provider: aws
    options:
      class: dev
      version: 8.4.6

services:
  web:
    build: .
    port: 3000
    resources:
      - database
```

### Production with High Availability

```yaml
resources:
  database:
    type: mysql
    provider: aws
    options:
      class: large
      version: 8.4.6
      durable: true

services:
  web:
    build: .
    port: 3000
    resources:
      - database
```

### Legacy MySQL 8.0

```yaml
resources:
  legacy-db:
    type: mysql
    provider: aws
    options:
      class: medium
      version: 8.0.44

services:
  web:
    build: .
    port: 3000
    resources:
      - legacy-db
```

## Environment Variables

When linked to a service, the following environment variables are injected (assuming resource name `database`):

```
DATABASE_URL=mysql://username:password@host.name:3306/database
DATABASE_USER=username
DATABASE_PASS=password
DATABASE_HOST=host.name
DATABASE_PORT=3306
DATABASE_NAME=database
```

## Command Line Interface

### Launch Console

```bash
$ convox cloud resources console database -a myapp -i my-machine
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 1
Server version: 8.4.6 MySQL Community Server
Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.
```

### Export Data

```bash
$ convox cloud resources export database -a myapp -i my-machine --file backup.sql
Exporting data from database... OK
```

### Import Data

```bash
$ convox cloud resources import database -a myapp -i my-machine --file backup.sql
Importing data to database... OK
```

### Proxy Connection

```bash
$ convox cloud resources proxy database -a myapp -i my-machine
Proxying localhost:3306 to host.name:3306
```

## Next Steps

- [Cloud Databases Overview](/cloud/databases) - General database documentation
- [Sizing and Pricing](/cloud/databases/sizing-and-pricing) - Detailed pricing information