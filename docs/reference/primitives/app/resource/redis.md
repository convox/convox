---
title: "Redis"
slug: redis
url: /reference/primitives/app/resource/redis
---
# Redis

## Definition

A Redis Resource is defined in [`convox.yml`](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).

```yaml
resources:
  cache:
    type: redis
services:
  web:
    resources:
      - cache
```

## Containerized Options

By default, Convox runs Redis as a container inside your Rack. Containerized Redis does not use persistent storage -- data is lost if the container restarts.

```yaml
resources:
  cache:
    type: redis
    options:
      version: "7.2"
```

| Attribute   | Type   | Default  | Description                  |
| ----------- | ------ | -------- | ---------------------------- |
| **version** | string | `4.0.10` | The Redis Docker image tag   |

> Specify a recent Redis version for production use. The default `4.0.10` is the template fallback; most deployments should set an explicit version such as `7.2`.

## AWS ElastiCache Managed Redis Resources

Convox allows the creation of Redis resources via AWS ElastiCache. This provides enhanced durability, automatic failover, and managed service benefits. Use `elasticache-redis` as the resource type:

```yaml
resources:
  cache:
    type: elasticache-redis
    options:
      class: cache.t3.micro
      version: "7.1"
      deletionProtection: true
      durable: true
      encrypted: true
      autoMinorVersionUpgrade: true
services:
  web:
    resources:
      - cache
```

### ElastiCache Features

- **Durable Redis Instances**: Support for creating durable Redis instances with automatic failover using Multi-AZ replication.
- **Import Existing ElastiCache Redis Instance**: Import an existing AWS ElastiCache Redis instance into a Convox rack for management or access via linking.

### ElastiCache Configuration Options

| Attribute                   | Type    | Default      | Description                                                                                         |
| --------------------------- | ------- | ------------ | --------------------------------------------------------------------------------------------------- |
| **class**                   | string  | **Required** | The compute and memory capacity of the cache instance (e.g., `cache.t3.micro`, `cache.m5.large`)   |
| **version**                 | string  | **Required** | The version of the Redis engine (e.g., `7.1`, `6.2`)                                              |
| **deletionProtection**      | boolean | `false`      | Whether to enable deletion protection. Managed by Convox (not an AWS feature). Prevents the resource from being removed if accidentally deleted from `convox.yml` |
| **durable**                 | boolean | `false`      | Whether to enable automatic failover (Multi-AZ)                                                    |
| **encrypted**               | boolean | `false`      | Whether to enable encryption at rest. Immutable after creation                                     |
| **nodes**                   | int     |              | The number of cache clusters (read replicas) in the replication group                              |
| **autoMinorVersionUpgrade** | boolean | `false`      | Whether to allow automatic minor version upgrades                                                  |
| **import**                  | string  |              | The replication group identifier for importing an existing ElastiCache instance                     |

## Command Line Interface

### Listing Resources
```bash
$ convox resources -a myapp
NAME   TYPE              URL
cache  elasticache-redis redis://hostname:port
```

### Getting Information about a Resource
```bash
$ convox resources info cache -a myapp
Name  cache
Type  elasticache-redis
URL   redis://hostname:port
```

### Getting the URL for a Resource
```bash
$ convox resources url cache -a myapp
redis://hostname:port
```

### Starting a Proxy to a Resource
```bash
$ convox resources proxy cache -a myapp
Proxying localhost:6379 to hostname:port
```
> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.
