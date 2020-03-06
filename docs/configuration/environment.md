# Environment Variables

Convox encourages the use of environment variables for managing application secrets. Using environment
variables allows you to keep secrets out of your codebase and to have different configuration values
for different deployments (i.e. staging and production).

## Definition

Environment variables that will be used by the application are defined in `convox.yml`:

### Application Level

Environment variables defined at the top level affect every service in the application.

    environment:
      - ENCRYPTION_KEY
    services:
      web:
        environment:
          - ALLOWED_IPS
          - COOKIE_SECRET
      worker:
        environment:
          - QUEUE

This application would require four environment variables to be set: `ALLOWED_IPS`, `COOKIE_SECRET`, `ENCRYPTION_KEY`, and `QUEUE`.

The `ENCRYPTION_KEY` variable will be available to both services.

### Service Level

Environment variables can be defined for each [Service](../reference/primitives/app/service.md).

    services:
      web:
        environment:
          - ALLOWED_IPS
          - COOKIE_SECRET
      worker:
        environment:
          - QUEUE

This application would require three environment variables to be set: `ALLOWED_IPS`, `COOKIE_SECRET`, and `QUEUE`.

> Environment variables defined at the service level will only be available to that service. In the example above,
> the `web` service would not have a value set for the `QUEUE` environment variable as that variable is only defined
> on the `worker` service.


### Default Values

You can set a default value for any environment variable in the manifest:

    environment:
      - QUEUE=main

### Interpolation

You can also use environment variables to add dynamic configuration to your `convox.yml`:

    services:
      web:
        health: ${HEALTH_CHECK_PATH}

## Configuration

You can set values for your environment variables using `convox env set`:

    $ convox env set ALLOWED_IPS=1.2.3.4 COOKIE_SECRET=foo QUEUE=main
    Setting ALLOWED_IPS, COOKIE_SECRET, QUEUE... OK
    Release: RABCDEFGHI

Setting environment variables will cause a new [Release](../reference/primitives/app/release.md) to be created. In order to deploy
your changes you will need to promote this release.

    $ convox releases promote RABCDEFGHI
    Promoting RABCDEFGHI... OK

> Environment variables must be defined in the `convox.yml` for their values to be populated on a
> [Service](../reference/primitives/app/service.md).

## System Variables

The following environment variables are automatically set by Convox.

| Name                | Description                                                                                   |
| ------------------- | --------------------------------------------------------------------------------------------- |
| `APP`               | Name of the [App](../reference/primitives/app)                                                |
| `BUILD`             | ID of the currently-promoted [Build](../reference/primitives/app/build.md)                    |
| `BUILD_DESCRIPTION` | Description of the currently-promoted [Build](../reference/primitives/app/build.md)           |
| `PORT`              | The value of the `port:` attribute for this [Service](../reference/primitives/app/service.md) |
| `RACK`              | The name of the [Rack](../reference/primitives/rack.md)                                       |
| `RELEASE`           | ID of the currently-promoted [Release](../reference/primitives/app/release.md)                |
| `SERVICE`           | Name of the [Service](../reference/primitives/app/service.md)                                 |
