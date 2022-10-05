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
### Types

The following Resource types are currently available:

* `mariadb`
* `memcached`
* `mysql`
* `postgis`
* `postgres`
* `redis`

## Linking

Linking a Resource to a [Service](/reference/primitives/app/service) causes an environment variable to be injected into [Processes](/reference/primitives/app/process)
of that [Service](/reference/primitives/app/service) based on the name of the Resource.

The credential details will be stored in the environment variables, you can use the FQDN (URL) or each credential separately.

For example, a `postgres` resource named `main` (as in the example above) would injected like this:

```
MAIN_URL=postgres://username:password@host.name:port/database`
MYDB_USER=username
MYDB_PASS=password
MYDB_HOST=host.name
MYDB_PORT=port
MYDB_NAME=database
```

## Overlays

By default, any Resources you define will be satisfied by starting a containerized version on your [Rack](/reference/primitives/rack).  This allows you to get up and running as quickly as possible and also provides a low cost solution and more effective usage of your Rack.

In your production environment, or for particular usage requirements, you may wish to replace the containerized Resources with a managed cloud service for durability.  For instance, on AWS you may wish to utilise RDS to provide you with a Database, or on GCP you may wish to use Memorystore in place of a containerized Redis instance.

Resource Overlays provide you with a simple and effective way to maintain the cheaper and efficient containerized Resources on the environments you wish, whilst easily switching them out for the cloud-provider managed services on those environments that require them.

If you wish to replace any of those containerized Resources on a Rack, to stop them being initiated, you can manually set a matching environment variable is on your [App](/reference/primitives/app).  The corresponding Resource will then not be started by Convox on that Rack.

```sh
$ convox env set MAIN_URL=postgres://username:password@postgres–instance1.123456789012.us-east-1.rds.amazonaws.com:5432/database -r production-rack
Setting MAIN_URL... OK
Release: RABCDEFGHI
```

By doing this, a containerized `main` resource will now no longer be started on the `production-rack` for this app.  The service will instead communicate with the managed database instead.

### Specifying the environment variable

You can also specify the environment variable that should be used for linking in the `resources` attribute:
```html
    resources:
      main:
        type: postgres
    services:
      web:
        resources:
          - main:DIFFERENT_URL
```
This example would cause database URL to be injected as `DIFFERENT_URL`. This behavior disables the credential details, only the FQDN will be available.

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
