---
title: "Cloud vs Rack Comparison"
draft: false
slug: comparison
url: /cloud/comparison
---

# Cloud vs Rack Comparison

This comparison helps you choose between Convox Cloud (managed machines) and self-hosted Convox Racks based on your requirements.

## Summary

| Aspect | Convox Cloud | Self-Hosted Rack |
|--------|--------------|------------------|
| **Best For** | Rapid deployment, predictable costs | Full control, compliance needs |
| **Setup Time** | < 1 minute | 10-20 minutes |
| **Expertise Required** | Minimal | Moderate to Advanced |
| **Cost Model** | Fixed monthly per machine/database | Variable infrastructure costs |
| **Control Level** | Platform-managed | Full control |
| **Maintenance** | Zero | Regular updates required |

## Feature Comparison

### Infrastructure Management

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **AWS Account Required** | No | Yes |
| **VPC Management** | No | Yes |
| **Security Groups** | No | Yes |
| **IAM Roles** | No | Yes |
| **Subnet Configuration** | No | Yes |
| **NAT Gateways** | Managed | Self-managed |
| **Cluster Upgrades** | Automatic | Manual |
| **OS Patching** | Automatic | Manual/Automated |
| **Certificate Management** | Automatic | Automatic |

### Developer Experience

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **CLI Commands** | `convox cloud` | `convox` |
| **Deployment Speed** | Fast (shared infra) | Fast (dedicated infra) |
| **Build Environment** | Managed pool | Configurable |
| **Local Development** | Same as rack | Identical |
| **convox.yml Support** | Full | Full |
| **Environment Variables** | Yes | Yes |
| **Rolling Deployments** | Yes | Yes |
| **Rollback Support** | Yes | Yes |
| **Log Retention** | 7 days | Configurable |
| **Monitoring** | Basic | Full CloudWatch/Custom |

### Database Resources

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Managed RDS** | Yes (Cloud Databases) | Yes (RDS Resources) |
| **PostgreSQL** | Yes | Yes |
| **MySQL** | Yes | Yes |
| **MariaDB** | Yes | Yes |
| **Read Replicas** | No | Yes |
| **Custom Instance Types** | No (fixed classes) | Yes |
| **Custom Storage Sizes** | No (fixed per class) | Yes |
| **Snapshot Restore** | No | Yes |
| **Database Import** | No | Yes |
| **ElastiCache** | No | Yes |
| **Containerized DBs** | Yes | Yes |

### Operational Capabilities

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Kubectl Access** | No | Yes |
| **SSH to Nodes** | No | Yes |
| **Custom Node Types** | No | Yes |
| **Spot Instances** | No | Yes |
| **GPU Support** | No | Yes |
| **Custom AMIs** | No | Yes |
| **Cluster Autoscaling** | No | Configurable |
| **Pod Autoscaling** | Yes | Yes |
| **Custom Operators** | No | Yes |

### Networking

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Load Balancer Type** | Managed ALB | ALB/NLB/CLB |
| **Static IPs** | No | Yes |
| **Custom Domains** | Yes | Yes |
| **SSL Termination** | Yes | Yes |
| **VPC Peering** | No | Yes |
| **Private Link** | No | Yes |
| **Internal Load Balancers** | No | Yes |
| **Network Policies** | No | Yes |

### Storage

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Persistent Volumes** | No | Yes |
| **EFS Support** | No | Yes |
| **EBS Volumes** | No | Yes |
| **S3 Access** | Via credentials | Yes |

### Security and Compliance

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Network Isolation** | Namespace | Full VPC |
| **Pod Security Policies** | Platform-managed | Customizable |
| **Secrets Management** | Environment vars | Multiple options |
| **RBAC** | Platform-managed | Full control |
| **Audit Logging** | Basic | CloudTrail/Custom |
| **Custom Security Groups** | No | Yes |
| **WAF Integration** | No | Yes |

## Cost Analysis

### Convox Cloud Pricing

**Machines:**

| Machine Size | Monthly Cost |
|--------------|--------------|
| X-Small | $12 |
| Small | $25 |
| Medium | $75 |
| Large | $150 |

**Cloud Databases:**

| Database Class | Monthly Cost |
|----------------|--------------|
| dev | $19 |
| small | $39 |
| medium | $99 |
| large | $199 |

Enabling `durable: true` doubles the database cost.

### Self-Hosted Rack Costs (AWS)

Variable costs based on usage:

| Component | Typical Monthly Cost | Notes |
|-----------|---------------------|-------|
| EC2 Instances (3x t3.small) | ~$45 | Minimum HA setup |
| EKS Cluster | $73 | Fixed cost |
| Load Balancer | ~$25 | Plus data transfer |
| NAT Gateway | ~$45 | Plus data transfer |
| EBS Storage | ~$10 | 100 GB |
| Data Transfer | ~$20+ | Varies by usage |
| RDS (db.t3.micro) | ~$15+ | If using managed DB |
| **Total** | **~$228+** | Minimum production |

### Cost Comparison by Scale

| Workload | Convox Cloud | Self-Hosted Rack |
|----------|--------------|------------------|
| Dev/Test | $12-31 | ~$100 |
| Small Production | $64-103 | ~$228 |
| Medium Production | $174-273 | ~$400 |
| Large Production | $349-548 | ~$800+ |

## Scalability Limits

| Metric | Convox Cloud | Self-Hosted Rack |
|--------|--------------|------------------|
| Max Services/Machine | 20 | Unlimited |
| Max Processes/Service | 10 | Node limits |
| Max Apps/Machine | 10 | Unlimited |
| Build Concurrency | Queued | Configurable |
| Network Bandwidth | Shared | Dedicated |
| IOPS | Shared | Dedicated |

## Use Case Recommendations

### Choose Convox Cloud When:

- Need to deploy in minutes, not hours
- Limited DevOps resources
- Fixed monthly budgets
- Standard web applications
- Small development teams
- SaaS applications

### Choose Self-Hosted Rack When:

- Need Kubernetes access
- Compliance requirements (HIPAA, PCI DSS)
- Service mesh implementations
- Multi-region deployments
- GPU workloads
- VPC peering required
- Direct RDS/ElastiCache access with advanced features
- Very large applications

## Migration Paths

### Cloud to Rack

When to migrate:
- Outgrowing Cloud limitations
- Compliance requirements emerge
- Need for advanced features

Migration effort: **Low**
- Same application format
- Export/import supported
- 1-2 hours typical

### Rack to Cloud

When to migrate:
- Reducing operational overhead
- Simplifying management

Migration effort: **Low-Medium**
- Remove unsupported features
- Adjust for limitations
- 2-4 hours typical

## Hybrid Approach

Consider using both Cloud and Rack:

### Development on Cloud
- X-Small machines for developers
- Dev database class
- Cost-effective testing

### Production on Rack
- Full control and compliance
- Advanced features
- Custom configuration

### Example Setup
```bash
# Development - Create machine via Console first
$ convox cloud deploy -i dev

# Production
$ convox rack install aws production
$ convox deploy -r production
```

## Recommendation Summary

### Small Teams/Startups
**Recommend: Convox Cloud**
- Fastest time to market
- Predictable costs
- Minimal operational overhead

### Growing Companies
**Recommend: Hybrid Approach**
- Cloud for development/staging
- Rack for production
- Gradual migration path

### Enterprises
**Recommend: Self-Hosted Rack**
- Full control and compliance
- Advanced integration options
- Custom configuration

### Agencies
**Recommend: Convox Cloud**
- Quick client deployments
- Predictable billing
- Multi-tenant efficiency

## Getting Started

### Try Convox Cloud
```bash
# Create machine via Console at console.convox.com
$ convox cloud apps create my-app -i my-first-machine
$ convox cloud deploy -i my-first-machine -a my-app
```

### Try Self-Hosted Rack
```bash
# Create Runtime Integration then Install Rack via Console
$ convox apps create my-app -r my-first-rack
$ convox deploy -r my-first-rack -a my-app
```

## Support

- Sales: sales@convox.com
- Support: cloud-support@convox.com
- Community: community.convox.com