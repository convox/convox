# Postgres

## Definition

A Postgres Resource is defined in [`convox.yml`](../../../../configuration/convox-yml.md) and linked to one or more [Services](../service.md).

    resources:
      main:
        type: postgres
    services:
      web:
        resources:
          - main

## Options

A Postgres Resource can have the following options configured for it (default values are shown):

    resources:
      main:
        type: postgres
        options:
          version: 10.5
          storage: 10
