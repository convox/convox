---
title: "Redis"
draft: false
slug: Redis
url: /reference/primitives/app/resource/redis
---
# Redis

## Definition

A Redis Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).
```html
    resources:
      main:
        type: redis
    services:
      web:
        resources:
          - main
```
## Options

A Redis Resource can have the following options configured for it (default values are shown):
```html
    resources:
      main:
        type: redis
        options:
          version: 4.0.10
```