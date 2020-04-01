# Redis

## Definition

A Redis Resource is defined in [`convox.yml`](../../../../configuration/convox-yml.md) and linked to one or more [Services](../service.md).

    resources:
      main:
        type: redis
    services:
      web:
        resources:
          - main

## Options

A Redis Resource can have the following options configured for it (default values are shown):

    resources:
      main:
        type: redis
        options:
          version: 4.0.10
