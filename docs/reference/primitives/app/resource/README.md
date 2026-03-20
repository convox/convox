---
title: "Resource"
slug: resource
url: /reference/primitives/app/resource
---
# Resource

A Resource is a network-accessible external service such as a database or cache. Convox supports three resource deployment models:

- **Containerized**: Runs the resource inside your Rack cluster. Fast to set up, low cost, ideal for development and staging.
- **AWS Managed (RDS/ElastiCache)**: Provisions a fully managed AWS service. Use for production workloads requiring durability, backups, and high availability.
- **Convox Cloud Databases**: Managed databases available on [Convox Cloud](/cloud/databases) with simplified configuration and pricing.

## Definition

A Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).

```yaml
resources:
  main:
    type: postgres
services:
  web:
    resources:
      - main
```

## Types

### Containerized

- `mariadb`
- `memcached`
- `mysql`
- [`postgis`](/reference/primitives/app/resource/postgis)
- `postgres`
- `redis`

### AWS Managed (RDS/ElastiCache)

- `rds-mariadb`
- `rds-mysql`
- `rds-postgres`
- `elasticache-redis`
- `elasticache-memcached`

See [AWS RDS Managed Database Resources](#aws-rds-managed-database-resources) and [AWS ElastiCache Redis and Memcached Resources](#aws-elasticache-redis-and-memcached-resources) below for configuration details.

## Linking

Linking a Resource to a [Service](/reference/primitives/app/service) causes an environment variable to be injected into [Processes](/reference/primitives/app/process) of that [Service](/reference/primitives/app/service) based on the name of the Resource.

The credential details will be stored in the environment variables, and you can use the FQDN (URL) or each credential separately.

For example, a `postgres` resource named `main` (as in the example above) would be injected like this:

```text
MAIN_URL=postgres://username:password@host.name:port/database
MAIN_USER=username
MAIN_PASS=password
MAIN_HOST=host.name
MAIN_PORT=port
MAIN_NAME=database
```

## Custom Images

You can also pass a compatible custom image for all resource types.

To use a custom image, include the `image` field in your resource configuration:

```yaml
resources:
  main:
    type: postgres
    image: pgvector/pgvector:pg16
services:
  web:
    resources:
      - main
```

Note:

1. Always include the image tag when specifying a custom image. If no tag is provided, the `latest` tag will be used. Ensure that the specified image has a `latest` tag or provide a specific tag to avoid errors.
2. The image field takes precedence over the version field. If both are specified, the version field will be ignored.

Example:

```yaml
resources:
  myRedis:
    type: redis
    image: custom-redis-image:6.2
    options:
      version: 6.0
```

In this example, a custom Redis image named `custom-redis-image` with tag `6.2` will be used.

## Overlays

By default, any Resources you define will be satisfied by starting a containerized version on your [Rack](/reference/primitives/rack). This allows you to get up and running as quickly as possible and provides a low-cost solution and more effective usage of your Rack.

In your production environment, or for particular usage requirements, you may wish to replace the containerized Resources with a managed cloud service for durability. For instance, on AWS you may wish to utilize RDS to provide you with a Database, or replace a containerized Redis instance with an ElastiCache-managed resource.

Resource Overlays provide you with a simple and effective way to maintain the cheaper and efficient containerized Resources on the environments you wish, while switching them out for the cloud-provider managed services on those environments that require them.

### Example: Development vs Production Resources

In development, you might use a containerized Postgres resource:

```yaml
resources:
  database:
    type: postgres
services:
  web:
    resources:
      - database
```

For production, you can overlay this with an AWS RDS managed database without changing your application code:

```yaml
resources:
  database:
    type: rds-postgres
    options:
      class: db.m5.large
      storage: 100
      encrypted: true
      durable: true
services:
  web:
    resources:
      - database
```

The environment variable injected into your service (e.g., `DATABASE_URL`) uses the same format in both cases, so your application connects to either resource transparently.

If you wish to replace any of those containerized Resources on a Rack, to stop them being initiated, you can manually set a matching environment variable on your [App](/reference/primitives/app). The corresponding Resource will then not be started by Convox on that Rack.

```bash
$ convox env set MAIN_URL=postgres://username:password@postgres-instance1.123456789012.us-east-1.rds.amazonaws.com:5432/database -r production-rack
Setting MAIN_URL... OK
Release: RABCDEFGHI
```

By doing this, a containerized `main` resource will now no longer be started on the `production-rack` for this app. The service will instead communicate with the managed database.

## AWS RDS Managed Database Resources

In addition to containerized resources, Convox allows the creation of database resources (mariadb, mysql, and postgres) via AWS RDS. This integration provides enhanced durability and managed service benefits. Below are the basic configurations required for using AWS RDS resources.

### Defining AWS RDS Resources

AWS RDS resources are specified with a `rds-` prefix followed by the database type (e.g., `rds-mariadb`, `rds-mysql`, `rds-postgres`). Here is a general example of how to define AWS RDS resources:

```yaml
resources:
  database:
    type: rds-postgres
    options:
      storage: 100
      class: db.t3.large
      version: "16"
services:
  web:
    resources:
      - database
```

For detailed configuration options and defaults for each type of AWS RDS resource, refer to the specific resource documentation pages:

- [MariaDB](/reference/primitives/app/resource/mariadb)
- [MySQL](/reference/primitives/app/resource/mysql)
- [PostgreSQL](/reference/primitives/app/resource/postgres)

### Advisory

If an application is deleted, it will delete its created RDS databases. We advise enabling `deletionProtection` for any production or critical databases to avoid any accidental removal. If a database is imported, the database will not be removed if the application is deleted and it will need to be manually deleted.

## RDS Features

### Read Replicas

Read replicas allow you to create read-only copies of your database to improve read scalability and reliability.

To configure a read replica, use the `readSourceDB` option to point to another database using the name you've chosen in the `convox.yml`:

```yaml
resources:
  mydb:
    type: rds-postgres
    options:
      storage: 100
      class: db.t3.large
      version: "16"
  dbrr:
    type: rds-postgres
    options:
      readSourceDB: "#convox.resources.mydb"
services:
  web:
    resources:
      - mydb
      - dbrr
```

**Immutable Attributes for Read Replicas**:
- Engine version
- Storage type
- Storage encryption
- Storage volume

To promote a read replica to an active primary DB, remove the `readSourceDB` option from the `convox.yml` and redeploy.

Resource linking works the same with read replicas, meaning environment variables for both primary and read replica databases will be created based on their respective YAML names.

### Database Import

Database import allows you to integrate any RDS managed database or Elasticache into Convox, whether it was initially created by Convox or not.

```yaml
resources:
  mydb-import:
    type: rds-postgres
    options:
      import: mydb-rtest-rc6-7924r-postgres-rds-check
      masterUserPassword: ${MYDBPASS}
services:
  web:
    resources:
      - mydb-import
```

**Usage**:
- Set the `import` option with the `masterUserPassword` before deploying.
- As long as the `import` option is set, the database will act as a passive linked access database.
- Remove the `import` option and redeploy to manage the database through Convox.
- While `import` is set, no other configured options will be considered. If `import` is removed, the application will have management control over the RDS database, and the `masterUserPassword` no longer needs to be configured.

You can set the master user password using `convox env set` before deploying:

```bash
$ convox env set MYDBPASS=my_secure_password -a myapp
Setting MYDBPASS... OK
Release: RABCDEFGHI
```

### Snapshot Support

Snapshots allow you to restore a database from a specific point in time.

```yaml
resources:
  db-from-snap:
    type: rds-postgres
    options:
      storage: 10
      snapshot: test-v3-rds-snapshot-postgres
      version: "16"
services:
  web:
    resources:
      - db-from-snap
```

**Usage**:
- Set the `snapshot` option with the snapshot identifier and ensure the `version` matches the engine version of the snapshot.
- Remove the `snapshot` option and redeploy to enable options management from the `convox.yml`.

**Immutable Attributes for Snapshots**:
- Storage encryption
- Engine version
- Storage volume

## AWS ElastiCache Redis and Memcached Resources

Convox supports native AWS ElastiCache Redis and Memcached instances for high-performance caching solutions. These managed cache instances can be defined and linked to services similarly to other managed resources.

### Defining AWS ElastiCache Resources

AWS ElastiCache resources are specified with an `elasticache-` prefix followed by the cache type (`redis` or `memcached`). Below are examples of defining both Redis and Memcached resources:

**Redis Example**:
```yaml
resources:
  cache:
    type: elasticache-redis
    options:
      class: cache.t3.micro
      version: 6.2
services:
  web:
    resources:
      - cache
```

**Memcached Example** (the `nodes` parameter must be set):
```yaml
resources:
  cache:
    type: elasticache-memcached
    options:
      version: 1.6.6
      class: cache.t4g.micro
      nodes: 1
services:
  web:
    resources:
      - cache
```

> **Note:** The `nodes` option is required for Memcached and must be set to the desired number of nodes for the Memcached cluster.

For detailed configuration options and examples for each type of AWS Elasticache resource, refer to the specific resource documentation pages:

- [Redis](/reference/primitives/app/resource/redis)
- [Memcached](/reference/primitives/app/resource/memcached)

## Command Line Interface

### Listing Resources
```bash
$ convox resources -a myapp
NAME  TYPE      URL
main  postgres  postgres://username:password@host.name:port/database
```

### Getting Information about a Resource
```bash
$ convox resources info main -a myapp
Name  main
Type  postgres
URL   postgres://username:password@host.name:port/database
```

### Getting the URL for a Resource
```bash
$ convox resources url main -a myapp
postgres://username:password@host.name:port/database
```

### Launching a Console
```bash
$ convox resources console main -a myapp
psql (11.5 (Debian 11.5-1+deb10u1), server 10.5 (Debian 10.5-2.pgdg90+1))
Type "help" for help.
database=#
```

### Starting a Proxy to a Resource
```bash
$ convox resources proxy main -a myapp
Proxying localhost:5432 to host.name:port
```
> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.

### Exporting Data from a Resource
```bash
$ convox resources export main -f /tmp/db.sql
Exporting data from main... OK
```

### Importing Data to a Resource
```bash
$ convox resources import main -f /tmp/db.sql
Importing data to main... OK
```

## See Also

- [Convox Cloud Databases](/cloud/databases) for managed databases with simplified pricing on Convox Cloud
- [Environment Variables](/configuration/environment) for configuring resource connection strings
