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
    $ convox registries add registry.example.org user password
    Adding registry... OK
```

Use `https://index.docker.io/v1/` for DockerHub.

## Listing Registries

```html
    $ convox registries
    SERVER                       USERNAME
    registry.example.org          user
```

## Deleting Registries

```html
    $ convox registries remove registry.example.org
    Removing registry... OK
```
