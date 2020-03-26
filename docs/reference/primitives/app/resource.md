# Resource

A Resource is a network-accessible external service.

## Definition

A Resource is defined in [`convox.yml`](../../../configuration/convox-yml.md) and linked to one or more [Services](service.md).

    resources:
      main:
        type: postgres
    services:
      web:
        resources:
          - main

### Types

The following Resource types are currently available:

* `memcached`
* `mysql`
* `postgres`
* `redis`

## Linking

Linking a Resource to a [Service](service.md) causes an environment variable to be injected into [Processes](process.md)
of that [Service](service.md) based on the name of the Resource.

The URL presented by this environment variable will contain everything you need to connect, including authentication details.

For example, a `postgres` resource named `main` (as in the example above) would injected like this:

`MAIN_URL=postgres://username:password@host.name:port/database`

### Specifying the environment variable

You can also specify the environment variable that should be used for linking in the `resources` attribute:

    resources:
      main:
        type: postgres
    services:
      web:
        resources:
          - main:DIFFERENT_URL

This example would cause database URL to be injected as `DIFFERENT_URL`

## Command Line Interface

### Listing Resources

    $ convox resources -a myapp
    NAME  TYPE      URL
    main  postgres  postgres://username:password@host.name:port/database

### Getting Information about a Resource

    $ convox resources info main -a myapp
    Name  main
    Type  postgres
    URL   postgres://username:password@host.name:port/database

### Getting the URL for a Resource

    $ convox resources url main -a myapp
    postgres://username:password@host.name:port/database

### Launching a Console

    $ convox resources console main -a myapp
    psql (11.5 (Debian 11.5-1+deb10u1), server 10.5 (Debian 10.5-2.pgdg90+1))
    Type "help" for help.
    database=#

### Starting a Proxy to a Resource

    $ convox resources proxy main -a myapp
    Proxying localhost:5432 to host.name:port

> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.

### Exporting Data from a Resource

    $ convox resources export main -f /tmp/db.sql
    Exporting data from main... OK

### Importing Data to a Resource

    $ convox resources import main -f /tmp/db.sql
    Importing data to main... OK