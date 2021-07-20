# MariaDB

## Definition

A MariaDB Resource is defined in [`convox.yml`](../../../../configuration/convox-yml.md) and linked to one or more [Services](../service.md).

    resources:
      main:
        type: mariadb
    services:
      web:
        resources:
          - main

## Options

A MariaDB Resource can have the following options configured for it (default values are shown):

    resources:
      main:
        type: mariadb
        options:
          version: 10.6.0
          storage: 10
