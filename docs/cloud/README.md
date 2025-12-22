---
title: "Convox Cloud"
draft: false
slug: cloud
url: /cloud
---

# Convox Cloud

Convox Cloud is a fully-managed, multi-tenant platform built on the same technology as Convox Rack. It provides predictable pricing and instant deployment capabilities without requiring you to manage your own infrastructure.

## What is Convox Cloud?

Convox Cloud introduces **Machines** - dedicated compute resources that host your applications in a managed environment. Unlike traditional Convox Racks that run in your own cloud account, Machines run on Convox-managed infrastructure, eliminating the need for cloud provider setup, Kubernetes management, or infrastructure maintenance.

Convox Cloud also provides **Cloud Databases** - fully managed RDS database instances (PostgreSQL, MySQL, MariaDB) with automated backups and optional high availability.

### Key Benefits

- **Zero Infrastructure Management**: No AWS account, no Kubernetes cluster, no maintenance overhead
- **Instant Setup**: Deploy applications in seconds
- **Predictable Pricing**: Fixed per-machine and per-database pricing with no hidden infrastructure costs
- **Full Convox Compatibility**: Use the same `convox.yml` configuration and deployment workflows
- **Managed Databases**: Fully managed PostgreSQL, MySQL, and MariaDB with automated backups

## Cloud vs Self-Hosted Racks

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Setup Time** | Instant | 10-20 minutes |
| **Cloud Account Required** | No | Yes |
| **Infrastructure Management** | Convox-managed | Self-managed |
| **Pricing Model** | Per machine/database | Infrastructure costs |
| **Kubernetes Access** | Restricted | Full access |
| **Customization** | Limited | Full control |
| **Managed Databases** | Yes (RDS) | Containerized or self-managed |
| **Best For** | Rapid deployment, predictable costs | Complete control, custom requirements |

## Machines

A Machine is a slice of dedicated compute resources (vCPU and RAM) that hosts your applications. Machines come in predefined sizes:

| Size | vCPU | RAM | Monthly Price |
|------|------|-----|---------------|
| **X-Small** | 0.5 | 1 GB | $12 |
| **Small** | 1 | 2 GB | $25 |
| **Medium** | 2 | 4 GB | $75 |
| **Large** | 4 | 8 GB | $150 |

Each machine can host multiple services as long as their combined resource usage doesn't exceed the machine's capacity.

## Cloud Databases

Cloud Databases provide fully managed RDS instances with automated backups. Supported engines:

- **PostgreSQL** - Versions 14.x through 18.x
- **MySQL** - Versions 8.0.x and 8.4.x
- **MariaDB** - Versions 10.6.x through 11.8.x

| Class | vCPU | RAM | Storage | Monthly Price |
|-------|------|-----|---------|---------------|
| **dev** | 1.0 | 1 GB | 20 GB | $19 |
| **small** | 2.0 | 2 GB | 50 GB | $39 |
| **medium** | 2.0 | 4 GB | 100 GB | $99 |
| **large** | 2.0 | 8 GB | 250 GB | $199 |

All database classes include 7-day automated backups. Enable `durable: true` for Multi-AZ failover (doubles cost).

## Quick Start

1. **Install the Convox CLI** (if not already installed):
```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

2. **Login to Convox**:
```bash
$ convox login console.convox.com
```

3. **Create a Machine**:
   - Log into the [Convox Console](https://console.convox.com)
   - Navigate to the Cloud Machines page
   - Click "New Machine" and select your desired size and region

4. **Deploy an Application**:
```bash
$ convox cloud apps create my-app -i my-machine
$ convox cloud deploy -i my-machine -a my-app
```

## Example Configuration

```yaml
resources:
  database:
    type: postgres
    provider: aws
    options:
      class: small
      version: 17.5

services:
  web:
    build: .
    port: 3000
    scale:
      count: 1-3
      cpu: 250
      memory: 512
    resources:
      - database
```

## Core Concepts

### Machines
Pre-configured compute units that provide CPU, memory, and storage for your applications. Machines are fully managed and automatically maintained.

### Cloud Databases
Fully managed RDS database instances. Define databases in your `convox.yml` with `provider: aws` to use Cloud Databases instead of containerized databases.

### Applications
Standard Convox applications that run on machines. Applications use the same `convox.yml` format and support the same features as rack-based deployments.

### Build Isolation
Builds run in separate, ephemeral environments to prevent resource contention.

### Automatic Updates
System components and security patches are automatically applied.

## Use Cases

Convox Cloud is suitable for:

- **Startups**: Deploy quickly without infrastructure overhead
- **Development Teams**: Focus on building features
- **Agencies**: Deploy client applications with predictable pricing
- **Side Projects**: Cost-effective hosting
- **Staging Environments**: Quickly spin up review apps and staging environments

## Next Steps

- [Getting Started](/cloud/getting-started) - Step-by-step guide to deploying your first application
- [Machine Management](/cloud/machines) - Learn how to create and manage machines
- [Cloud Databases](/cloud/databases) - Configure managed databases
- [CLI Reference](/cloud/cli-reference) - Complete reference for `convox cloud` commands
- [Migration Guide](/cloud/migration-guide) - Migrate from other platforms

## Support

- Documentation: [docs.convox.com](https://docs.convox.com)
- Community: [community.convox.com](https://community.convox.com)
- Support: cloud-support@convox.com