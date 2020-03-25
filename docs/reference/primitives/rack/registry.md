# Registry

External Docker image repository.  When you install a [Rack](../rack) an internal Registry is created for use by the Rack.  You can add external private Registries to use when refering to private images in your [Build](../app/build.md)

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