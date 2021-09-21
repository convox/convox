---
title: "Private Registries"
draft: false
slug: Private Registries
url: /configuration/private-registries
---
# Private Registries

Convox can pull base images from private registries during the build process.

## Command Line Interface

### Adding a Registry
```html
    $ convox registries add registry.example.org username password
    Adding registry... OK
```
### Listing Registries
```html
    $ convox registries
    SERVER                USERNAME
    registry.example.org  username
```
### Removing a Registry
```html
    $ convox registries remove registry.example.org
    Removing registry... OK
```