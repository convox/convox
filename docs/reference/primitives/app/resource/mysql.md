# MySQL

## Definition

A MySQL Resource is defined in [`convox.yml`](../../../../configuration/convox-yml.md) and linked to one or more [Services](../service.md).

    resources:
      main:
        type: mysql
    services:
      web:
        resources:
          - main

## Options

A MySQL Resource can have the following options configured for it (default values are shown):

    resources:
      main:
        type: mysql
        options:
          version: 5.7.23
          storage: 10
