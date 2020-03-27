# Memcached

## Definition

A Memcached Resource is defined in [`convox.yml`](../../../../configuration/convox-yml.md) and linked to one or more [Services](../service.md).

    resources:
      main:
        type: memcached
    services:
      web:
        resources:
          - main

## Options

A Memcached Resource can have the following options configured for it (default values are shown):

    resources:
      main:
        type: memcached
        options:
          version: 1.4.34
