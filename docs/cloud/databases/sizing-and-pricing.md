---
title: "Database Sizing and Pricing"
draft: false
slug: sizing-and-pricing
url: /cloud/databases/sizing-and-pricing
---

# Database Sizing and Pricing

Convox Cloud Databases use hourly billing with fixed monthly pricing. Pay only for the hours you use with no hidden infrastructure costs.

## Database Classes

| Class | vCPU | RAM | Storage | Monthly Price | Hourly Price |
|-------|------|-----|---------|---------------|--------------|
| **dev** | 1.0 | 1 GB | 20 GB | $19 | ~$0.026 |
| **small** | 2.0 | 2 GB | 50 GB | $39 | ~$0.053 |
| **medium** | 2.0 | 4 GB | 100 GB | $99 | ~$0.135 |
| **large** | 2.0 | 8 GB | 250 GB | $199 | ~$0.272 |

## Multi-AZ (Durable) Pricing

Enabling `durable: true` creates a standby replica for automatic failover. This doubles the monthly cost:

| Class | Standard | With Durable |
|-------|----------|--------------|
| **dev** | $19/mo | $38/mo |
| **small** | $39/mo | $78/mo |
| **medium** | $99/mo | $198/mo |
| **large** | $199/mo | $398/mo |

## Features by Class

### Dev ($19/mo)

**Specifications:**
- 1.0 vCPU
- 1 GB RAM
- 20 GB Storage

**Features:**
- Development databases
- Staging environments
- Sandbox testing
- 7-day automated backups

**Best For:** Development, testing, proof of concepts, personal projects

### Small ($39/mo)

**Specifications:**
- 2.0 vCPU
- 2 GB RAM
- 50 GB Storage

**Features:**
- Production-ready performance
- Light to moderate traffic
- Optional Multi-AZ failover
- 7-day automated backups

**Best For:** Small production applications, APIs, light traffic sites

### Medium ($99/mo)

**Specifications:**
- 2.0 vCPU
- 4 GB RAM
- 100 GB Storage

**Features:**
- Growing applications
- Higher connection limits
- Read replica support
- 7-day automated backups

**Best For:** Growing applications, moderate traffic, multiple services

### Large ($199/mo)

**Specifications:**
- 2.0 vCPU
- 8 GB RAM
- 250 GB Storage

**Features:**
- High-traffic production
- Maximum connection capacity
- Enterprise workloads
- 7-day automated backups

**Best For:** High-traffic production, resource-intensive workloads, enterprise applications

## All Classes Include

- 7-day automated backups
- Automatic minor version upgrades
- SSL/TLS encryption in transit
- Encryption at rest
- PostgreSQL, MySQL, and MariaDB support

## Pricing Examples

### Development Environment

A development setup with a single database:

| Resource | Class | Monthly Cost |
|----------|-------|--------------|
| Machine | X-Small | $12 |
| PostgreSQL | dev | $19 |
| **Total** | | **$31** |

### Small Production Application

A production app with a durable database:

| Resource | Class | Monthly Cost |
|----------|-------|--------------|
| Machine | Small | $25 |
| PostgreSQL (durable) | small | $78 |
| **Total** | | **$103** |

### Multi-Database Production

A production environment with multiple databases:

| Resource | Class | Monthly Cost |
|----------|-------|--------------|
| Machine | Medium | $75 |
| PostgreSQL (primary, durable) | medium | $198 |
| MySQL (legacy) | small | $39 |
| **Total** | | **$312** |

### Enterprise Setup

A high-traffic production environment:

| Resource | Class | Monthly Cost |
|----------|-------|--------------|
| Machine | Large | $150 |
| PostgreSQL (durable) | large | $398 |
| **Total** | | **$548** |

## Billing Details

### Billing Cycle
- Hourly billing
- Monthly invoicing
- Pro-rated for partial months

### What's Included
- All database features for the selected class
- Automated backups (7-day retention)
- Encryption at rest and in transit
- Automatic minor version upgrades

## Choosing the Right Class

### Start with Dev When:
- Building or testing new features
- Running staging environments
- Learning Convox Cloud
- Budget is a primary concern

### Upgrade to Small When:
- Moving to production
- Handling real user traffic
- Need production-ready performance

### Upgrade to Medium When:
- Application is growing
- Need more connections
- Running multiple services against one database

### Upgrade to Large When:
- High-traffic production workloads
- Enterprise requirements
- Maximum performance needed

## FAQ

### Can I change database class after creation?

Database class changes are not currently supported. To change class, export your data, create a new database with the desired class, and import your data.

### Is there a free tier for databases?

There is no free tier for Cloud Databases. The dev class at $19/mo is the most economical option.

### What happens if my database runs out of storage?

Storage is fixed per class. Monitor your usage and upgrade to a larger class before reaching storage limits.

### Are backups included?

Yes, 7-day automated backups are included with all database classes at no additional cost.

### Can I take manual backups?

Yes, use the `convox cloud resources export` command to export your database at any time.

## Next Steps

- [Cloud Databases Overview](/cloud/databases) - General database documentation
- [PostgreSQL](/cloud/databases/postgres) - PostgreSQL-specific documentation
- [MySQL](/cloud/databases/mysql) - MySQL-specific documentation
- [MariaDB](/cloud/databases/mariadb) - MariaDB-specific documentation