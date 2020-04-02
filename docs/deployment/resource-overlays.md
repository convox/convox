# Resource Overlays

By default, any [Resources](../reference/primitives/app/resource) you define will be run in containers on your [Rack](../reference/primitives/rack).  This allows you to get up and running as quickly as possible and also provides a low cost solution and more effective usage of your Rack.  

In your production environment, or for particular usage requirements, you may find it beneficial to replace the containerised Resources with other external managed services.  For instance, on AWS you may wish to utilise RDS to provide you with a Database, or on GCP you may wish to use Memorystore in place of a containerised Redis instance.

Resource Overlays provide you with a simple and effective way to maintain the cheaper and efficient containerised Resources on the environments you wish, whilst easily switching them out for the cloud-provider managed services on those environments that require them.

## Configuration

Resource Overlays make use of the standardised environment variables injected into your [Services](../reference/primitives/app/service.md) by Convox.  Given an example `convox.yml` as below:

```yml
resources:
  database:
    type: postgres
      options:
        storage: 200
  queue:
    type: redis
services:
  api:
    resources:
      - database
      - queue
...
```

When this is deployed onto a Rack, two Resource containers will be initiated, one running Postgres, one running Redis.  The `api` service will have two environment variables injected into it; `DATABASE_URL` and `QUEUE_URL`, each containing the connection string necessary to communicate with those Resources.

## Overlays

If you wish to replace any of those containerised Resources on a Rack, to stop them being initiated, then if a matching environment variable is set on an [App](../reference/primitives/app) then the corresponding Resource will not be started by Convox on that Rack.

```sh
$ convox env set DATABASE_URL=postgres://username:password@postgres–instance1.123456789012.us-east-1.rds.amazonaws.com:5432/database -r production-rack
Setting DATABASE_URL... OK
Release: RABCDEFGHI
```

By doing this, a containerised `database` resource will now no longer be started on the `production-rack` for this app.  The service will instead communicate with the managed service instead.  

The Overlay works on the name of Resource and it's subsequent environment variable, which by default is constructed by capitalising the Resource name and appending `_URL`.

You can also specify a different environment variable when linking a resource in the following way:

```yml
resources:
  main:
    type: postgres
services:
  api:
    resources:
      - main:DIFFERENT_URL
```

This example would cause the Resource URL to be injected as `DIFFERENT_URL` rather than `MAIN_URL`.
