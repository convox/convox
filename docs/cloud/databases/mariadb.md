---
title: "MariaDB"
draft: false
slug: mariadb
url: /cloud/databases/mariadb
---

# MariaDB

Convox Cloud provides fully managed MariaDB databases through AWS RDS.

## Definition

```yaml
resources:
  database:
    type: mariadb
    provider: aws
    options:
      class: small
      version: 11.4.8

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
| **version** | string | `11.4.8` | MariaDB version |
| **durable** | boolean | `false` | Enable Multi-AZ failover |

## Supported Versions

| Version | Status |
|---------|--------|
| 11.8.5 | Available |
| 11.8.3 | Available |
| 11.4.9 | Available |
| 11.4.8 | Available |
| 11.4.7 | Available |
| 10.11.15 | Available |
| 10.11.14 | Available |
| 10.11.13 | Available |
| 10.6.24 | Available |
| 10.6.23 | Available |
| 10.6.22 | Available |

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
    type: mariadb
    provider: aws
    options:
      class: dev
      version: 11.4.8

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
    type: mariadb
    provider: aws
    options:
      class: large
      version: 11.4.8
      durable: true

services:
  web:
    build: .
    port: 3000
    resources:
      - database
```

### Multiple MariaDB Versions

```yaml
resources:
  primary:
    type: mariadb
    provider: aws
    options:
      class: medium
      version: 11.4.8
      durable: true
      
  legacy:
    type: mariadb
    provider: aws
    options:
      class: small
      version: 10.6.24

services:
  web:
    build: .
    port: 3000
    resources:
      - primary
      
  legacy-service:
    build: ./legacy
    resources:
      - legacy
```

## Environment Variables

When linked to a service, the following environment variables are injected (assuming resource name `database`):

```
DATABASE_URL=mariadb://username:password@host.name:3306/database
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
Welcome to the MariaDB monitor.  Commands end with ; or \g.
Your MariaDB connection id is 1
Server version: 11.4.8-MariaDB MariaDB Server
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