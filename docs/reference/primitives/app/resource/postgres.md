---
title: "Postgres"
slug: postgres
url: /reference/primitives/app/resource/postgres
---
# Postgres

## Definition

A Postgres Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).

```yaml
resources:
  database:
    type: postgres
services:
  web:
    resources:
      - database
```

## Containerized Options

By default, Convox runs Postgres as a container inside your Rack. This is fast to set up and ideal for development and staging environments.

```yaml
resources:
  database:
    type: postgres
    options:
      version: "16"
      storage: 20
```

| Attribute   | Type   | Default | Description                                     |
| ----------- | ------ | ------- | ----------------------------------------------- |
| **version** | string | `10.5`  | The PostgreSQL Docker image tag                 |
| **storage** | int    | `10`    | The amount of persistent storage (in GB)        |

> Specify a recent PostgreSQL version for production use. The default `10.5` is the template fallback; most deployments should set an explicit version such as `16` or `17`.

## AWS RDS Managed Postgres Resources

Convox allows the creation of Postgres resources via AWS RDS. This provides enhanced durability, automated backups, and managed service benefits. Use `rds-postgres` as the resource type:

```yaml
resources:
  database:
    type: rds-postgres
    options:
      class: db.m5.large
      storage: 100
      version: "16"
      deletionProtection: true
      durable: true
      encrypted: true
      backupRetentionPeriod: 7
      preferredBackupWindow: 02:00-03:00
      preferredMaintenanceWindow: sun:05:00-sun:06:00
services:
  web:
    resources:
      - database
```

### RDS Features

- **Read Replica Support**: Configure read replicas for improved read scalability and reliability. Read replicas can be promoted to active primaries if needed.
- **Snapshot Restoration**: Restore from a snapshot to create a new database instance with your desired specifications.
- **Import Existing RDS Database**: Import existing AWS RDS databases into a Convox rack for management or access via linking.

### RDS Configuration Options

| Attribute                     | Type    | Default          | Description                                                                                                   |
| ----------------------------- | ------- | ---------------- | ------------------------------------------------------------------------------------------------------------- |
| **class**                     | string  | **Required**     | The compute and memory capacity of the DB instance (e.g., `db.t3.micro`, `db.m5.large`)                     |
| **version**                   | string  | **Required**     | The version of the database engine (e.g., `16`, `15`, `14`)                                                  |
| **storage**                   | int     | `20`             | The amount of storage (in GB) to allocate for the DB instance                                                |
| **encrypted**                 | boolean | `false`          | Whether to enable storage encryption. Immutable after creation                                                |
| **deletionProtection**        | boolean | `false`          | Whether to enable deletion protection for the DB instance                                                    |
| **durable**                   | boolean | `false`          | Whether to create a Multi-AZ DB instance for high availability                                               |
| **backupRetentionPeriod**     | int     | `1`              | The number of days for which automated backups are retained. Set to `0` to disable automated backups         |
| **preferredBackupWindow**     | string  | AWS managed      | The daily time range for automated backups in UTC (format: `hh24:mi-hh24:mi`, at least 30 minutes)          |
| **preferredMaintenanceWindow**| string  | AWS managed      | The weekly time range for system maintenance in UTC (format: `ddd:hh24:mi-ddd:hh24:mi`)                     |
| **iops**                      | int     |                  | The amount of provisioned IOPS                                                                               |
| **port**                      | int     | `5432`           | The port on which the database accepts connections. Immutable after creation                                  |
| **masterUserPassword**        | string  | Convox generated | The password for the master user. Set as an environment variable to avoid hardcoding                         |
| **allowMajorVersionUpgrade**  | boolean | `true`           | Whether major version upgrades are allowed                                                                   |
| **autoMinorVersionUpgrade**   | boolean | `true`           | Whether minor version upgrades are applied automatically                                                     |
| **readSourceDB**              | string  |                  | The source database identifier for creating a read replica                                                   |
| **import**                    | string  |                  | The database identifier for importing an existing RDS instance. Requires `masterUserPassword`                |
| **snapshot**                  | string  |                  | The snapshot identifier for restoring from a snapshot                                                        |

## Command Line Interface

### Listing Resources
```bash
$ convox resources -a myapp
NAME      TYPE          URL
database  rds-postgres  postgres://username:password@host.name:port/database
```

### Getting Information about a Resource
```bash
$ convox resources info database -a myapp
Name  database
Type  rds-postgres
URL   postgres://username:password@host.name:port/database
```

### Getting the URL for a Resource
```bash
$ convox resources url database -a myapp
postgres://username:password@host.name:port/database
```

### Launching a Console
```bash
$ convox resources console database -a myapp
psql (16.2 (Debian 16.2-1.pgdg120+2), server 16.2)
Type "help" for help.
database=#
```

### Starting a Proxy to a Resource
```bash
$ convox resources proxy database -a myapp
Proxying localhost:5432 to host.name:port
```
> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.

### Exporting Data from a Resource
```bash
$ convox resources export database -f /tmp/db.sql
Exporting data from database... OK
```

### Importing Data to a Resource
```bash
$ convox resources import database -f /tmp/db.sql
Importing data to database... OK
```
