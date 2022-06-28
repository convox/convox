---
title: "Postgis"
draft: false
slug: Postgis
url: /reference/primitives/app/resource/postgis
---
# Postgis

## Definition

A Postgis Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).
```html
    resources:
      main:
        type: postgis
    services:
      web:
        resources:
          - main
```
## Options

A Postgis Resource can have the following options configured for it (default values are shown):
```html
    resources:
      main:
        type: postgis
        options:
          version: 10-3.2
          storage: 10
```