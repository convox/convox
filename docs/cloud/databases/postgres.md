---
title: "PostgreSQL"
draft: false
slug: postgres
url: /cloud/databases/postgres
---

# PostgreSQL

Convox Cloud provides fully managed PostgreSQL databases through AWS RDS.

## Definition

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

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| **class** | string | `dev` | Database size: `dev`, `small`, `medium`, or `large` |
| **version** | string | `17.5` | PostgreSQL version |
| **durable** | boolean | `false` | Enable Multi-AZ failover |

## Supported Versions

| Version | Status |
|---------|--------|
| 18 | Available |
| 17.7 | Available |
| 17.6 | Available |
| 17.5 | Available |
| 16.11 | Available |
| 16.9 | Available |
| 16.1 | Available |
| 15.15 | Available |
| 15.14 | Available |
| 15.13 | Available |
| 14.19 | Available |
| 14.18 | Available |
| 14.2 | Available |

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

### Production with High Availability

```yaml
resources:
  database:
    type: postgres
    provider: aws
    options:
      class: large
      version: 17.5
      durable: true

services:
  web:
    build: .
    port: 3000
    resources:
      - database
```

### Multiple PostgreSQL Databases

```yaml
resources:
  primary:
    type: postgres
    provider: aws
    options:
      class: medium
      version: 17.5
      durable: true
      
  analytics:
    type: postgres
    provider: aws
    options:
      class: small
      version: 16.11

services:
  web:
    build: .
    port: 3000
    resources:
      - primary
      
  reporting:
    build: ./reporting
    resources:
      - analytics
```

## Environment Variables

When linked to a service, the following environment variables are injected (assuming resource name `database`):

```
DATABASE_URL=postgres://username:password@host.name:5432/database
DATABASE_USER=username
DATABASE_PASS=password
DATABASE_HOST=host.name
DATABASE_PORT=5432
DATABASE_NAME=database
```

## Command Line Interface

### Launch Console

```bash
$ convox cloud resources console database -a myapp -i my-machine
psql (17.5)
Type "help" for help.
database=#
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
Proxying localhost:5432 to host.name:5432
```

## Next Steps

- [Cloud Databases Overview](/cloud/databases) - General database documentation
- [Sizing and Pricing](/cloud/databases/sizing-and-pricing) - Detailed pricing information