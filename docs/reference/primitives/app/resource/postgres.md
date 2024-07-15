---
title: "Postgres"
draft: false
slug: Postgres
url: /reference/primitives/app/resource/postgres
---
# Postgres

## Definition

A Postgres Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).

```html
resources:
  database:
    type: postgres
services:
  web:
    resources:
      - database
```

## Options

A Postgres Resource can have the following options configured for it (default values are shown):

```html
resources:
  database:
    type: postgres
    options:
      version: 13
      storage: 10
```

### AWS RDS Managed Postgres Resources

In addition to containerized resources, Convox v3 allows the creation of Postgres resources via AWS RDS. This provides enhanced durability and managed service benefits. Below is a general example of how to define AWS RDS resources:

```html
resources:
  database:
    type: rds-postgres
    options:
      class: db.m5.large
      storage: 100
      version: 13
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

### Features

- **Read Replica Support**: AWS RDS resources support integrated AWS read replicas. You can configure read replicas for improved read scalability and reliability. Additionally, read replicas can be promoted to active primaries if needed.
- **Snapshot Restoration**: Easily restore from a snapshot to create a new database instance with your desired specifications.
- **Import Existing RDS Database**: Import existing AWS RDS databases into a Convox rack for management or access via linking.

### Configuration Options

Below is a chart of configuration options available for AWS RDS Postgres resources:

| Attribute                     | Type    | Default       | Description                                                                                                   |
| ----------------------------- | ------- | ------------- | ------------------------------------------------------------------------------------------------------------- |
| **class**                     | string  | `db.t3.micro` | The compute and memory capacity of the DB instance.                                                           |
| **encrypted**                 | boolean | `false`       | Whether to enable storage encryption.                                                                         |
| **deletionProtection**        | boolean | `false`       | Whether to enable deletion protection for the DB instance.                                                    |
| **durable**                   | boolean | `false`       | Whether to create a Multi-AZ DB instance.                                                                     |
| **storage**                   | int     | `20`          | The amount of storage (in GB) to allocate for the DB instance.                                                |
| **version**                   | string  | `13`          | The version of the database engine.                                                                           |
| **preferredBackupWindow**     | string  | `AWS managed` | The daily time range during which automated backups are created if automated backups are enabled, using the `backupRetentionPeriod` option. Must be in the format hh24:mi-hh24:mi, in UTC, at least 30 minutes, and must not conflict with the preferred maintenance window. If not set, AWS decides based on usage. |
| **backupRetentionPeriod**     | int     | `1`           | The number of days for which automated backups are retained. Setting this parameter to a positive number enables backups. Setting this parameter to 0 disables automated backups.             |
| **iops**                      | int     | `AWS managed` | The amount of provisioned IOPS.                                                                               |
| **port**                      | int     | `5432`        | The port on which the database accepts connections.                                                           |
| **masterUserPassword**        | string  | `Convox managed` | The password for the master user. Should be set as an environment variable to avoid hardcoding.               |
| **allowMajorVersionUpgrade**  | boolean | `true`        | Whether major version upgrades are allowed.                                                                   |
| **autoMinorVersionUpgrade**   | boolean | `true`        | Whether minor version upgrades are applied automatically.                                                     |
| **preferredMaintenanceWindow**| string  | `AWS managed` | The weekly time range during which system maintenance can occur. Must be in the format ddd:hh24:mi-ddd:hh24:mi, in UTC, at least 30 minutes, and must not conflict with the preferred backup window. If not set, AWS decides based on usage. |
| **readSourceDB**              | string  | ``            | The source database identifier for creating a read replica.                                                   |
| **import**                    | string  | ``            | The database identifier used for database import. Requires the correct `masterUserPassword` option set.       |
| **snapshot**                  | string  | ``            | The snapshot identifier for restoring from a snapshot.                                                        |

*Note: The `readSourceDB` option is used for the read replica feature.*

*Note: The `import` option requires the correct `masterUserPassword` to be set for importing a database.*

*Note: The `masterUserPassword` should be set as an environment variable to avoid hardcoding the password.*

### Command Line Interface

#### Listing Resources
```html
$ convox resources -a myapp
NAME      TYPE          URL
database  rds-postgres  postgres://username:password@host.name:port/database
```

#### Getting Information about a Resource
```html
$ convox resources info database -a myapp
Name  database
Type  rds-postgres
URL   postgres://username:password@host.name:port/database
```

#### Getting the URL for a Resource
```html
$ convox resources url database -a myapp
postgres://username:password@host.name:port/database
```

#### Launching a Console
```html
$ convox resources console database -a myapp
psql (13.3 (Debian 13.3-1.pgdg90+1), server 13.3 (Debian 13.3-2.pgdg90+1))
Type "help" for help.
database=#
```

#### Starting a Proxy to a Resource
```html
$ convox resources proxy database -a myapp
Proxying localhost:5432 to host.name:port
```
> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.

#### Exporting Data from a Resource
```html
$ convox resources export database -f /tmp/db.sql
Exporting data from database... OK
```

#### Importing Data to a Resource
```html
$ convox resources import database -f /tmp/db.sql
Importing data to database... OK
```
