---
title: "Private Registries"
slug: private-registries
url: /configuration/private-registries
---
# Private Registries

Convox can pull base images from private registries during the build process.

## Command Line Interface

### Adding a Registry
```bash
    $ convox registries add registry.example.org username password
    Adding registry... OK
```
### Listing Registries
```bash
    $ convox registries
    SERVER                USERNAME
    registry.example.org  username
```
### Removing a Registry
```bash
    $ convox registries remove registry.example.org
    Removing registry... OK
```