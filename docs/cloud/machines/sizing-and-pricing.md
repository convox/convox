---
title: "Sizing and Pricing"
draft: false
slug: sizing-and-pricing
url: /cloud/machines/sizing-and-pricing
---

# Sizing and Pricing

Convox Cloud offers simple, predictable pricing based on machine size. Choose the right size for your workload and pay a fixed monthly price with no hidden infrastructure costs.

## Machine Tiers

### Available Sizes

| Machine Size | vCPU | RAM | Monthly Price | Hourly Price* | Free Hours** |
|--------------|------|-----|---------------|---------------|--------------|
| **X-Small** | 0.5 | 1 GB | $12 | $0.016 | 250/month |
| **Small** | 1 | 2 GB | $25 | $0.034 | - |
| **Medium** | 2 | 4 GB | $75 | $0.102 | - |
| **Large** | 4 | 8 GB | $150 | $0.205 | - |

*Hourly pricing for reference; billing is monthly
**X-Small tier includes 250 free hours per month

### Resource Specifications

#### X-Small (Development Tier)
- **vCPU**: 0.5 (500 millicores)
- **RAM**: 1 GB
- **Network**: 1 Gbps shared
- **Storage**: 10 GB
- **Build Resources**: Shared build pool
- **Best For**: Development, testing, proof of concepts, personal projects

#### Small (Starter Production)
- **vCPU**: 1
- **RAM**: 2 GB
- **Network**: Up to 5 Gbps
- **Storage**: 20 GB
- **Build Resources**: Dedicated build allocation
- **Best For**: Production applications, small to medium traffic sites, APIs

#### Medium (Growth Tier)
- **vCPU**: 2
- **RAM**: 4 GB
- **Network**: Up to 10 Gbps
- **Storage**: 40 GB
- **Build Resources**: Enhanced build allocation
- **Best For**: Growing applications, multiple services, moderate traffic

#### Large (Performance Tier)
- **vCPU**: 4
- **RAM**: 8 GB
- **Network**: Up to 10 Gbps
- **Storage**: 80 GB
- **Build Resources**: Priority build allocation
- **Best For**: High-traffic production, resource-intensive workloads, multiple applications

## Free Tier Details

### X-Small Free Hours

The X-Small tier includes 250 free hours per month, perfect for:
- Development and testing
- Hobby projects
- Learning Convox
- Staging environments

**How it works:**
- First 250 hours each month are free
- Additional hours billed at $0.016/hour
- Unused hours don't roll over
- Applies per account, not per machine

**Example usage:**
- Running 24/7 for ~11 days (250 hours) = FREE
- Running one machine full month (730 hours) = $7.89 for extra 480 hours

## Pricing Examples

### Single Service Application

**Small E-commerce Site**
- Machine: Small ($25/month)
- Services: Web + Background Worker
- Traffic: 10,000 visitors/month
- **Total: $25/month**

### Multi-Service Application

**SaaS Platform**
- Machine: Medium ($75/month)
- Services: Web + API + Worker + Cache
- Traffic: 50,000 visitors/month
- **Total: $75/month**

### High-Traffic Application

**Content Platform**
- Machine: Large ($150/month)
- Services: Web (3x) + API (2x) + Worker (2x) + Cache + Queue
- Traffic: 500,000 visitors/month
- **Total: $150/month**

## Billing Details

### Billing Cycle
- Monthly billing
- Pro-rated for partial months
- Automatic renewal

## Included Features

All machine tiers include:

### Infrastructure
- Managed Kubernetes cluster
- Automatic OS and security updates
- Built-in load balancing
- DDoS protection

### Developer Features
- Unlimited deployments
- Automatic SSL certificates
- Custom domains
- Environment variables
- Log access
- Basic monitoring

### Build Resources
- Isolated build environment
- Parallel builds (queued)
- Docker layer caching
- Private registry storage

### Support
- Community support
- Documentation
- CLI tools
- Web console access
- Premium Support Plan

## FAQ

### What happens if I exceed resource limits?
Services will be throttled or may experience performance degradation. Consider upgrading your machine size.

### Are there any hidden costs?
No. The monthly machine price includes all infrastructure, bandwidth (within reasonable limits), storage, and platform features.

### Can I pause a machine to save costs?
Currently, machines cannot be paused. For temporary environments, we recommend using the X-Small tier's free hours.  You can create an destoy machines at any time and will be billed on a pro-rated basis.

### How does billing work for autoscaling?
Autoscaling happens within your machine's resource limits. There are no additional charges for scaling services up or down within your machine.

## Next Steps

- [Machine Management](/cloud/machines) - Learn how to create and manage machines
- [Getting Started](/cloud/getting-started) - Deploy your first application
- [Limitations](/cloud/machines/limitations) - Understand platform constraints