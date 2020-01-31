# Private Registries

Convox can pull base images from private registries during the build process.

## Command Line Interface

### Adding a Registry

    $ convox registries add registry.example.org username password
    Adding registry... OK

### Listing Registries

    $ convox registries
    SERVER                USERNAME
    registry.example.org  username

### Removing a Registry

    $ convox registries remove registry.example.org
    Removing registry... OK