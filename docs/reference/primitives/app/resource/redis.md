---
title: "Redis"
draft: false
slug: Redis
url: /reference/primitives/app/resource/redis
---
# Redis

## Definition

A Redis Resource is defined in [```convox.yml```](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).

```
resources:
  main:
    type: redis
services:
  web:
    resources:
      - main
```

### AWS ElastiCache Managed Redis Resources

In addition to containerized resources, Convox v3 allows the creation of Redis resources via AWS ElastiCache. This provides enhanced durability and managed service benefits. Below is a general example of how to define AWS ElastiCache Redis resources:

```
resources:
  cache:
    type: elasticache-redis
    options:
      class: cache.t3.micro
      version: 6.2
      deletionProtection: true
      durable: true
      encrypted: true
      autoMinorVersionUpgrade: true
      transitEncryption: true
services:
  web:
    resources:
      - cache
```

### Features

- **Durable Redis Instances**: Support for creating durable Redis instances using AWS ElastiCache, which ensures data reliability and automatic failover.
- **Import Existing ElastiCache Redis Instance**: You can import an existing AWS ElastiCache Redis instance into a Convox rack for management or access via linking.

### Configuration Options

Below is a chart of configuration options available for AWS ElastiCache Redis resources:

| Attribute                   | Type    | Description                                                                                         |
| --------------------------- | ------- | --------------------------------------------------------------------------------------------------- |
| **class**                   | string  | The compute and memory capacity of the cache instance.                                               |
| **version**                 | string  | The version of the cache engine.                                                                     |
| **deletionProtection**      | boolean | Whether to enable deletion protection for the cache instance.                                        |
| **durable**                 | boolean | Whether to create a Multi-AZ cache instance.                                                         |
| **encrypted**               | boolean | Whether to enable encryption at rest for the cache instance.                                         |
| **transitEncryption**       | boolean | Whether to enable in-transit encryption for securing data flow between applications and cache.        |
| **autoMinorVersionUpgrade** | boolean | Whether to allow automatic minor version upgrades for the cache engine.                              |
| **import**                  | string  | The cache identifier used for cache import.                                                          |


> **Important:** The `deletionProtection` option is managed by Convox and is not an AWS feature. It is used to prevent the resource from being removed if accidentally deleted from the `convox.yml` file. If `deletionProtection` is enabled, the resource will not be deleted even if it is removed from the manifest.


### Command Line Interface

#### Listing Resources
```
$ convox resources -a myapp
NAME      TYPE       URL
cache     elasticache-redis  redis://hostname:port
```

#### Getting Information about a Resource
```
$ convox resources info cache -a myapp
Name  cache
Type  elasticache-redis
URL   redis://hostname:port
```

#### Getting the URL for a Resource
```
$ convox resources url cache -a myapp
redis://hostname:port
```

#### Starting a Proxy to a Resource
```
$ convox resources proxy cache -a myapp
Proxying localhost:6379 to hostname:port
```
> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.
