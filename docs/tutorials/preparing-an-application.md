---
order: 1
---

# Preparing An Application

In order to deploy you app on Convox you will need two things

* A `Dockerfile`

* A `convox.yml` manifest

## Dockerfile

If you have not already containerized your application it is a relatively straightforward process. You can start by cloning a Dockerfile from one of our [example apps](https://github.com/convox-examples/). If we don't have a Dockerfile for your chosen language or framework you can check the [Docker Hub](https://hub.docker.com/) to find a public image to use as a starting point. We also have a [guide](../configuration/dockerfile) for optimizing your Dockerfile to improve things like caching and build times.

## Convox.yml

Once you have your Dockerfile ready to go you need to create a convox.yml file. This file is a manifest that describes the components that make up your application. If you are familiar with using a docker-compose file you should find the convox.yml format to be very familiar. This guide will walk you through the basic components of the Convox manifest but you can find a complete specification [here](../configuration/convox-yml.md).

The Convox manifest has four major components of which only `services` is required. They are:

* Environment

* Resources

* Services

* Timers

### Environment

The environment section is where you can define any global environment variables that will be made available to any service in your application. Environment variables are generally used for environment specific configuration and secrets management and for this reason we generally recommend setting default values that would be appropriate for things like local development and storing your actual production values with `convox env set` or the convox environment control panel in the web console for each specific deployment of your app.

### Resources

Resources are network-accessible external services. Most often, resources are databases. You can see the list of currently supported resources [here](..reference/primitives/app/resource#types). When you specify a resource, Convox will pull a public docker image for the resource type and run that resource in a container inside your cluster. Any service which is [linked](https://docs.convox.com/reference/primitives/app/resource#linking) to a resource will automatically have a connection string URL for that resource injected as an environment variable at runtime. Convox resources are durable and can be used for production but if you would prefer to use a cloud specific database service such as RDS or Cloud SQL you can do so using resource [overlays](https://docs.convox.com/reference/primitives/app/resource#overlays) which allow you to specify an external service to use in place of the containerized resource for specific environments. This can be a great cost savings technique if for example you want to run a containerized Postgres database in your dev and staging environments but use RDS in production without needing to make any code changes or special configuration.

### Services

Services are the heart of your app. Services are scalable processes defined by a Dockerfile. Services can expose a port or set of ports and can be manual or automatically scaled. The most typical example of a service would be a web application such as a Ruby, Django, NodeJS or Go app that is accessed via an https connection. By default services with ports are publicly accessible but Convox also supports internal services that can only be accessed by other services running in the same Racks which can be useful for things like internal APIs. Convox also supports special use case services such as [agents](https://docs.convox.com/configuration/agents) and [singletons](https://docs.convox.com/reference/primitives/app/service). You can read the full specification for services [here](https://docs.convox.com/reference/primitives/app/service).

### Timers

A timer is effectively a cron job. With a timer you can specify a regular schedule to spawn a process and run a specific command. Timers must reference a service defined in your convox.yml which defines the process to be spawned. If you want to define a service that will be used exclusively as a timer job you can define that service with a [scale](https://docs.convox.com/deployment/scaling) of zero. You can read the full specification for timers [here](https://docs.convox.com/reference/primitives/app/timer)

### Convox.yml Example

Let's take a look at an example convox.yml. For this example we will specify a Django app with a web service, a celery worker service, and a timer that sends email reminders every five minutes using the worker service. For this example we will use a single Dockerfile for both services. This app also uses a Postgres database as a primary datastore and a Redis database as a celery broker.

```
environment:
  - DEVELOPMENT=True
resources:
  database:
    type: postgres
  broker:
    type: redis
services:
  web:
    build: .
    domain: ${DOMAIN}
    port: 8001
    resources:
      - database
      - broker
  worker:
    build: .
    command: celery -A web worker -l info
    resources:
      - database
      - broker
timers:
  reminderemails:
    schedule: "*/5 * * * *"
    command: python manage.py sendreminders
    service: worker
```

A few things to note here are that the web service specifies an internal port of 8001 which will be exposed externally on 443. The web service will also be made available on the domain(s) specified in the the DOMAIN environment variable and Convox will automatically provision a SSL [certificate](https://docs.convox.com/configuration/load-balancers#ssl-termination) for the specified domain. The worker service on the other hand does not expose any external ports. The worker service also uses a unique startup command while the web service uses the command specified within the Dockerfile. Hopefully this gives you a sense of what a typical convox.yml might look like for a production application.

## Going From docker-compose.yml to convox.yml

If you are already using `docker-compose.yml` for your application moving to a `convox.yml` is a relatively straightforward process. As an example take a look at the following `docker-compose.yml`

```
version: '2'
services:
  web:
    image: httpd
    ports:
      - 80:80
    volumes:
      - /tmp/something
    environment:
      - MY_ENVIRONMENT: development
    links:
      - supportservice
      - redis
  web_secondary:
    image: myusername/privateimage:latest
    command: override_command.sh
    ports:
      - 3001
    links:
      - redis
  redis:
    image: "redis:alpine"
  supportservice:
    build:
      context: .
      dockerfile: Dockerfile.support
      links:
        - postgres
  postgres:
    image: postgres:12

```

So here we have 5 different services defined, one a httpd service, one a private image with an overridden command, one a plain Redis service, and one we're building locally from a custom Dockerfile, with a Postgres database image thrown in for good measure.

The `convox.yml` manifest for this app would look like

```
resources:
  redis:
    type: redis
  postgres:
    type: postgres
    options:
      version: 12
services:
  web:
    image: httpd
    port: 80
    volumes:
      - /tmp/something
    enviroment:
      - MY_ENVIRONMENT=development
    resources:
      - redis
  web_secondary:
    image: myusername/privateimage:latest
    command: override_command.sh
    port: 3001
    resources:
      - redis
  supportservice:
    build:
      path: .
      manifest: Dockerfile.support
    resources:
      - postgres
```

Here you can see how we have turned the Redis and Postgres services into resources, which can be accessed by the other services. As we mentioned above, those resources will be run in containers backed by durable storage (by default) but if you would prefer to use a cloud provider service you can do so using [resource overlays](https://docs.convox.com/reference/primitives/app/resource#overlays). We have also removed the links directive as services in a Rack are able to communicate each other using our built-in [service discovery](https://docs.convox.com/configuration/service-discovery). Otherwise, you will find the manifests to be nearly identical.

## Next steps

* Check out: [Deploying an Application](deploying-an-application.md)
