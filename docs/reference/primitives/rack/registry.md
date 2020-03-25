# Registry

A Registry is a set of stored credentials for a private Docker registry that can be referenced during a [Build](../app/build.md).

## Adding Registries

    $ convox registries add index.docker.io/v1/ user password
    Adding registry... OK

## Listing Registries

    $ convox registries
    SERVER                       USERNAME
    index.docker.io/v1/          user

## Deleting Registries

    $ convox registries remove index.docker.io/v1/
    Removing registry... OK