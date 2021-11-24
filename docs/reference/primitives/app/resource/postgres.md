---
title: "Postgres"
draft: false
slug: Postgres
url: /reference/primitives/app/resource/postgres
---
# Postgres

## Definition

A Postgres Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).
```html
    resources:
      main:
        type: postgres
    services:
      web:
        resources:
          - main
```
## Options

A Postgres Resource can have the following options configured for it (default values are shown):
```html
    resources:
      main:
        type: postgres
        options:
          version: 10.5
          storage: 10
```