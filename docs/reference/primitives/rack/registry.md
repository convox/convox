# Registry

A reference to an external private Docker registry that you can reference in your [Builds](../app/build.md).

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