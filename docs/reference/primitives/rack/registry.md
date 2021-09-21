---
title: "Registry"
draft: false
slug: Registry
url: /reference/primitives/rack/registry
---
# Registry

A Registry is a set of stored credentials for a private Docker registry that can be referenced during a [Build](/reference/primitives/app/build).

## Adding Registries
```html
    $ convox registries add index.docker.io/v1/ user password
    Adding registry... OK
```
## Listing Registries
```html
    $ convox registries
    SERVER                       USERNAME
    index.docker.io/v1/          user
```
## Deleting Registries
```html
    $ convox registries remove index.docker.io/v1/
    Removing registry... OK
```