---
title: "Memcached"
slug: memcached
url: /reference/primitives/app/resource/memcached
---
# Memcached

## Definition

A Memcached Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).

```yaml
resources:
  cache:
    type: memcached
services:
  web:
    resources:
      - cache
```

## Containerized Options

By default, Convox runs Memcached as a container inside your Rack. Containerized Memcached does not use persistent storage -- cached data is lost if the container restarts.

```yaml
resources:
  cache:
    type: memcached
    options:
      version: "1.6"
```

| Attribute   | Type   | Default  | Description                      |
| ----------- | ------ | -------- | -------------------------------- |
| **version** | string | `1.4.34` | The Memcached Docker image tag   |

> Specify a recent Memcached version for production use. The default `1.4.34` is the template fallback; most deployments should set an explicit version such as `1.6`.

## AWS ElastiCache Managed Memcached Resources

Convox allows the creation of Memcached resources via AWS ElastiCache. This provides a managed, scalable caching cluster. Use `elasticache-memcached` as the resource type:

```yaml
resources:
  cache:
    type: elasticache-memcached
    options:
      class: cache.t3.micro
      version: "1.6.22"
      nodes: 2
      deletionProtection: true
services:
  web:
    resources:
      - cache
```

> The `nodes` option is required for Memcached and specifies the number of nodes in the cluster.

### ElastiCache Features

- **Scalable Memcached Clusters**: ElastiCache Memcached supports multiple nodes for improved scalability, making it ideal for distributed caching workloads.
- **Import Existing ElastiCache Memcached Instance**: Import an existing AWS ElastiCache Memcached instance into a Convox rack for management or access via linking.

### ElastiCache Configuration Options

| Attribute                   | Type    | Default      | Description                                                                                         |
| --------------------------- | ------- | ------------ | --------------------------------------------------------------------------------------------------- |
| **nodes**                   | int     | **Required** | The number of nodes in the Memcached cluster                                                        |
| **class**                   | string  | **Required** | The compute and memory capacity of the cache instance (e.g., `cache.t3.micro`, `cache.m5.large`)   |
| **version**                 | string  | **Required** | The version of the Memcached engine (e.g., `1.6.22`, `1.6.6`)                                     |
| **deletionProtection**      | boolean | `false`      | Whether to enable deletion protection. Managed by Convox (not an AWS feature). Prevents the resource from being removed if accidentally deleted from `convox.yml` |
| **encrypted**               | boolean | `false`      | Whether to enable encryption at rest                                                                |
| **autoMinorVersionUpgrade** | boolean | `false`      | Whether to allow automatic minor version upgrades                                                  |
| **import**                  | string  |              | The cache cluster identifier for importing an existing ElastiCache instance                         |

## Command Line Interface

### Listing Resources
```bash
$ convox resources -a myapp
NAME   TYPE                  URL
cache  elasticache-memcached memcached://hostname:port
```

### Getting Information about a Resource
```bash
$ convox resources info cache -a myapp
Name  cache
Type  elasticache-memcached
URL   memcached://hostname:port
```

### Getting the URL for a Resource
```bash
$ convox resources url cache -a myapp
memcached://hostname:port
```

### Starting a Proxy to a Resource
```bash
$ convox resources proxy cache -a myapp
Proxying localhost:11211 to hostname:port
```
> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.
