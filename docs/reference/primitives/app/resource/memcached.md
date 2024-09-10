---
title: "Memcached"
draft: false
slug: Memcached
url: /reference/primitives/app/resource/memcached
---
# Memcached

## Definition

A Memcached Resource is defined in [```convox.yml```](/configuration/convox-yml) and linked to one or more [Services](/reference/primitives/app/service).

```
resources:
  main:
    type: memcached
services:
  web:
    resources:
      - main
```


### AWS ElastiCache Managed Memcached Resources

In addition to containerized resources, Convox v3 allows the creation of Memcached resources via AWS ElastiCache. This provides enhanced durability and managed service benefits. Below is a general example of how to define AWS ElastiCache Memcached resources:

```
resources:
  cache:
    type: elasticache-memcached
    options:
      class: cache.t3.micro
      version: 1.6.6
      deletionProtection: true
      durable: true
      encrypted: true
      autoMinorVersionUpgrade: true
      nodes: 2
services:
  web:
    resources:
      - cache
```

> **Note:** The `nodes` option is required for Memcached and must be set to the desired number of nodes for the Memcached cluster.

### Features

- **Scalable Memcached Instances**: AWS ElastiCache Memcached supports multiple nodes for improved scalability, making it an ideal caching solution for distributed applications.
- **Import Existing ElastiCache Memcached Instance**: You can import an existing AWS ElastiCache Memcached instance into a Convox rack for management or access via linking.

### Configuration Options

Below is a chart of configuration options available for AWS ElastiCache Memcached resources:

| Attribute                | Type    | Description                                                                                       |
| ------------------------ | ------- | ------------------------------------------------------------------------------------------------- |
| **nodes**                | int     | **Required.** The number of nodes in the Memcached cluster.                                        |
| **class**                | string  | The compute and memory capacity of the cache instance.                                             |
| **version**              | string  | The version of the cache engine.                                                                   |
| **deletionProtection**   | boolean | Whether to enable deletion protection for the cache instance.                                      |
| **durable**              | boolean | Whether to create a Multi-AZ cache instance.                                                       |
| **encrypted**            | boolean | Whether to enable encryption at rest for the cache instance.                                       |
| **autoMinorVersionUpgrade** | boolean | Whether to allow automatic minor version upgrades for the cache engine.                           |
| **import**                    | string  | The cache identifier used for cache import. Requires the correct `masterUserPassword` option set.       |
| **masterUserPassword**        | string  | The password for the master user. Should be set as an environment variable to avoid hardcoding.               |

> **Important:** The `deletionProtection` option is managed by Convox and is not an AWS feature. It is used to prevent the resource from being removed if accidentally deleted from the `convox.yml` file. If `deletionProtection` is enabled, the resource will not be deleted even if it is removed from the manifest.

### Command Line Interface

#### Listing Resources
```
$ convox resources -a myapp
NAME      TYPE       URL
cache     elasticache-memcached  memcached://hostname:port
```

#### Getting Information about a Resource
```
$ convox resources info cache -a myapp
Name  cache
Type  elasticache-memcached
URL   memcached://hostname:port
```

#### Getting the URL for a Resource
```
$ convox resources url cache -a myapp
memcached://hostname:port
```

#### Starting a Proxy to a Resource
```
$ convox resources proxy cache -a myapp
Proxying localhost:11211 to hostname:port
```
> Proxying allows you to connect tools on your local machine to Resources running inside the Rack.
