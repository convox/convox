---
title: "Resource"
draft: false
slug: Resource
url: /reference/primitives/app/resource
---
# Resource

A Resource is a network-accessible external service.

## Definition

A Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).

```html
resources:
  main:
    type: postgres
services:
  web:
    resources:
      - main
```

## Types

The following Resource types are currently available:

* `mariadb`
* `memcached`
* `mysql`
* `postgis`
* `postgres`
* `redis`

## Linking

Linking a Resource to a [Service](/reference/primitives/app/service) causes an environment variable to be injected into [Processes](/reference/primitives/app/process) of that [Service](/reference/primitives/app/service) based on the name of the Resource.

The credential details will be stored in the environment variables, and you can use the FQDN (URL) or each credential separately.

For example, a `postgres` resource named `main` (as in the example above) would be injected like this:

```html
MAIN_URL=postgres://username:password@host.name:port/database
MAIN_USER=username
MAIN_PASS=password
MAIN_HOST=host.name
MAIN_PORT=port
MAIN_NAME=database
```

## Overlays

By default, any Resources you define will be satisfied by starting a containerized version on your [Rack](/reference/primitives/rack). This allows you to get up and running as quickly as possible and provides a low-cost solution and more effective usage of your Rack.

In your production environment, or for particular usage requirements, you may wish to replace the containerized Resources with a managed cloud service for durability. For instance, on AWS you may wish to utilize RDS to provide you with a Database, or on GCP you may wish to use Memorystore in place of a containerized Redis instance.

Resource Overlays provide you with a simple and effective way to maintain the cheaper and efficient containerized Resources on the environments you wish, while easily switching them out for the cloud-provider managed services on those environments that require them.

If you wish to replace any of those containerized Resources on a Rack, to stop them being initiated, you can manually set a matching environment variable on your [App](/reference/primitives/app). The corresponding Resource will then not be started by Convox on that Rack.

```html
$ convox env set MAIN_URL=postgres://username:password@postgres-instance1.123456789012.us-east-1.rds.amazonaws.com:5432/database -r production-rack
Setting MAIN_URL... OK
Release: RABCDEFGHI
```

By doing this, a containerized `main` resource will now no longer be started on the `production-rack` for this app. The service will instead communicate with the managed database.

## AWS RDS Managed Database Resources

In addition to containerized resources, Convox v3 allows the creation of database resources (mariadb, mysql, and postgres) via AWS RDS. This integration provides enhanced durability and managed service benefits. Below are the basic configurations required for using AWS RDS resources.

### Defining AWS RDS Resources

AWS RDS resources are specified with a `rds-` prefix followed by the database type (e.g., `rds-mariadb`, `rds-mysql`, `rds-postgres`). Here is a general example of how to define AWS RDS resources:

```html
resources:
  database:
    type: rds-postgres
    options:
      storage: 100
      class: db.t3.large
      version: 13
services:
  web:
    resources:
      - database
```

For detailed configuration options and defaults for each type of AWS RDS resource, refer to the specific resource documentation pages:

- [MariaDB](/reference/primitives/app/resource/mariadb/)
- [MySQL](/reference/primitives/app/resource/mysql/)
- [PostgreSQL](/reference/primitives/app/resource/postgres/)

### Advisory

If an application is deleted, it will delete its created RDS databases. We advise enabling `deletionProtection` for any production or critical databases to avoid any accidental removal. If a database is imported, the database will not be removed if the application is deleted and it will need to be manually deleted.


## RDS Features

### Read Replicas

Read replicas allow you to create read-only copies of your database to improve read scalability and reliability.

To configure a read replica, use the `readSourceDB` option to point to another database using the name you've chosen in the `convox.yml`:

```html
resources:
  mydb:
    type: rds-postgres
    options:
      storage: 100
      class: db.t3.large
      version: 13
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

Database import allows you to integrate any RDS managed database into Convox, whether it was initially created by Convox or not.

```html
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

```html
$ convox env set MYDBPASS=my_secure_password -a myapp
Setting MYDBPASS... OK
Release: RABCDEFGHI
```

### Snapshot Support

Snapshots allow you to restore a database from a specific point in time.

```html
resources:
  db-from-snap:
    type: rds-postgres
    options:
      storage: 10
      snapshot: test-v3-rds-snapshot-postgres
      version: 13
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

## Command Line Interface

### Listing Resources
```html
$ convox resources -a myapp
NAME  TYPE      URL
main  postgres  postgres://username:password@host.name:port/database
```

### Getting Information about a Resource
```html
$ convox resources info main -a myapp
Name  main
Type  postgres
URL   postgres://username:password@host.name:port/database
```

### Getting the URL for a Resource
```html
$ convox resources url main -a myapp
postgres://username:password@host.name:port/database
```

### Launching a Console
```html
$ convox resources console main -a myapp
psql (11.5 (Debian 11.5-1+deb10u1), server 10.5 (Debian 10.5-2.pgdg90+1))
Type "help" for help.
database=#
```

### Starting a Proxy to a Resource
```html
$ convox resources proxy main -a myapp
Proxying localhost:5432 to host.name:port
```
> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.

### Exporting Data from a Resource
```html
$ convox resources export main -f /tmp/db.sql
Exporting data from main... OK
```

### Importing Data to a Resource
```html
$ convox resources import main -f /tmp/db.sql
Importing data to main... OK
```
