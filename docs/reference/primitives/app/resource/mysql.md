---
title: "MySQL"
slug: mysql
url: /reference/primitives/app/resource/mysql
---
# MySQL

## Definition

A MySQL Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).

```yaml
resources:
  database:
    type: mysql
services:
  web:
    resources:
      - database
```

## Containerized Options

By default, Convox runs MySQL as a container inside your Rack. This is fast to set up and ideal for development and staging environments.

```yaml
resources:
  database:
    type: mysql
    options:
      version: "8.0"
      storage: 20
```

| Attribute   | Type   | Default  | Description                              |
| ----------- | ------ | -------- | ---------------------------------------- |
| **version** | string | `5.7.23` | The MySQL Docker image tag               |
| **storage** | int    | `10`     | The amount of persistent storage (in GB) |

> Specify a recent MySQL version for production use. The default `5.7.23` is the template fallback; most deployments should set an explicit version such as `8.0` or `8.4`.

## AWS RDS Managed MySQL Resources

Convox allows the creation of MySQL resources via AWS RDS. This provides enhanced durability, automated backups, and managed service benefits. Use `rds-mysql` as the resource type:

```yaml
resources:
  database:
    type: rds-mysql
    options:
      class: db.m5.large
      storage: 100
      version: "8.0"
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
- **Import Existing RDS Database**: Import existing AWS RDS databases into a Convox rack for management or access via linking.

### RDS Configuration Options

| Attribute                     | Type    | Default          | Description                                                                                                   |
| ----------------------------- | ------- | ---------------- | ------------------------------------------------------------------------------------------------------------- |
| **class**                     | string  | **Required**     | The compute and memory capacity of the DB instance (e.g., `db.t3.micro`, `db.m5.large`)                     |
| **version**                   | string  | **Required**     | The version of the database engine (e.g., `8.0`, `8.4`)                                                     |
| **storage**                   | int     | `20`             | The amount of storage (in GB) to allocate for the DB instance                                                |
| **encrypted**                 | boolean | `false`          | Whether to enable storage encryption. Immutable after creation                                                |
| **deletionProtection**        | boolean | `false`          | Whether to enable deletion protection for the DB instance                                                    |
| **durable**                   | boolean | `false`          | Whether to create a Multi-AZ DB instance for high availability                                               |
| **backupRetentionPeriod**     | int     | `1`              | The number of days for which automated backups are retained. Set to `0` to disable automated backups         |
| **preferredBackupWindow**     | string  | AWS managed      | The daily time range for automated backups in UTC (format: `hh24:mi-hh24:mi`, at least 30 minutes)          |
| **preferredMaintenanceWindow**| string  | AWS managed      | The weekly time range for system maintenance in UTC (format: `ddd:hh24:mi-ddd:hh24:mi`)                     |
| **iops**                      | int     |                  | The amount of provisioned IOPS                                                                               |
| **port**                      | int     | `3306`           | The port on which the database accepts connections. Immutable after creation                                  |
| **masterUserPassword**        | string  | Convox generated | The password for the master user. Set as an environment variable to avoid hardcoding                         |
| **allowMajorVersionUpgrade**  | boolean | `true`           | Whether major version upgrades are allowed                                                                   |
| **autoMinorVersionUpgrade**   | boolean | `true`           | Whether minor version upgrades are applied automatically                                                     |
| **readSourceDB**              | string  |                  | The source database identifier for creating a read replica                                                   |
| **import**                    | string  |                  | The database identifier for importing an existing RDS instance. Requires `masterUserPassword`                |

## Command Line Interface

### Listing Resources
```bash
$ convox resources -a myapp
NAME      TYPE       URL
database  rds-mysql  mysql://username:password@host.name:port/database
```

### Getting Information about a Resource
```bash
$ convox resources info database -a myapp
Name  database
Type  rds-mysql
URL   mysql://username:password@host.name:port/database
```

### Getting the URL for a Resource
```bash
$ convox resources url database -a myapp
mysql://username:password@host.name:port/database
```

### Launching a Console
```bash
$ convox resources console database -a myapp
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 1
Server version: 8.0.36 MySQL Community Server (GPL)
Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.
```

### Starting a Proxy to a Resource
```bash
$ convox resources proxy database -a myapp
Proxying localhost:3306 to host.name:port
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
