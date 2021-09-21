---
title: "MySQL"
draft: false
slug: MySQL
url: /reference/primitives/app/resource/mysql
---
# MySQL

## Definition

A MySQL Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).
```html
    resources:
      main:
        type: mysql
    services:
      web:
        resources:
          - main
```
## Options

A MySQL Resource can have the following options configured for it (default values are shown):
```html
    resources:
      main:
        type: mysql
        options:
          version: 5.7.23
          storage: 10
```