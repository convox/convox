---
title: "Memcached"
draft: false
slug: Memcached
url: /reference/primitives/app/resource/memcached
---
# Memcached

## Definition

A Memcached Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).
```html
    resources:
      main:
        type: memcached
    services:
      web:
        resources:
          - main
```
## Options

A Memcached Resource can have the following options configured for it (default values are shown):
```html
    resources:
      main:
        type: memcached
        options:
          version: 1.4.34
```