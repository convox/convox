---
title: "MariaDB"
draft: false
slug: MariaDB
url: /reference/primitives/app/resource/mariadb
---
# MariaDB

## Definition

A MariaDB Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).
```html
    resources:
      main:
        type: mariadb
    services:
      web:
        resources:
          - main
```
## Options

A MariaDB Resource can have the following options configured for it (default values are shown):
```html
    resources:
      main:
        type: mariadb
        options:
          version: 10.6.0
          storage: 10
```