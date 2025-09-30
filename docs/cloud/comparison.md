---
title: "Cloud vs Rack Comparison"
draft: false
slug: comparison
url: /cloud/comparison
---

# Cloud vs Rack Comparison

This comprehensive comparison helps you choose between Convox Cloud (managed machines) and self-hosted Convox Racks based on your specific requirements, technical expertise, and business needs.

## Executive Summary

| Aspect | Convox Cloud | Self-Hosted Rack |
|--------|--------------|------------------|
| **Best For** | Startups, rapid deployment, predictable costs | Enterprises, full control, compliance needs |
| **Setup Time** | < 1 minute | 10-20 minutes |
| **Expertise Required** | Minimal | Moderate to Advanced |
| **Cost Model** | Fixed monthly per machine | Variable infrastructure costs |
| **Control Level** | Platform-managed | Full control |
| **Maintenance** | Zero | Regular updates required |

## Detailed Feature Comparison

### Infrastructure Management

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **AWS Account Required** | ✗ | ✓ |
| **VPC Management** | ✗ | ✓ |
| **Security Groups** | ✗ | ✓ |
| **IAM Roles** | ✗ | ✓ |
| **Subnet Configuration** | ✗ | ✓ |
| **NAT Gateways** | Managed | Self-managed |
| **Internet Gateways** | Managed | Self-managed |
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
| **Environment Variables** | ✓ | ✓ |
| **Rolling Deployments** | ✓ | ✓ |
| **Rollback Support** | ✓ | ✓ |
| **Log Aggregation** | 7 days | Configurable |
| **Monitoring** | Basic | Full CloudWatch/Custom |

### Operational Capabilities

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Kubectl Access** | ✗ | ✓ |
| **SSH to Nodes** | ✗ | ✓ |
| **Custom Node Types** | ✗ | ✓ |
| **Spot Instances** | ✗ | ✓ |
| **GPU Support** | ✗ | ✓ |
| **Custom AMIs** | ✗ | ✓ |
| **Cluster Autoscaling** | ✗ | Configurable |
| **Pod Autoscaling** | ✓ | ✓ |
| **Custom Operators** | ✗ | ✓ |
| **Service Mesh** | ✗ | ✓ |

### Networking

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Load Balancer Type** | Managed ALB | ALB/NLB/CLB |
| **Static IPs** | ✗ | ✓ |
| **Custom Domains** | ✓ | ✓ |
| **SSL Termination** | ✓ | ✓ |
| **VPC Peering** | ✗ | ✓ |
| **Private Link** | ✗ | ✓ |
| **Internal Load Balancers** | ✗ | ✓ |
| **Network Policies** | ✗ | ✓ |
| **Service Discovery** | Internal only | Full |
| **Egress Control** | ✗ | ✓ |

### Storage and Databases

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Persistent Volumes** | ✗ | ✓ |
| **EFS Support** | ✗ | ✓ |
| **EBS Volumes** | ✗ | ✓ |
| **RDS Integration** | ✗ | ✓ |
| **ElastiCache** | ✗ | ✓ |
| **S3 Access** | ✗ | ✓ |
| **Containerized DBs** | ✓ | ✓ |
| **Backup Management** | Manual | Configurable |

### Security and Compliance

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Network Isolation** | Namespace | Full VPC |
| **Pod Security Policies** | Platform-managed | Customizable |
| **Secrets Management** | Environment vars | Multiple options |
| **RBAC** | Platform-managed | Full control |
| **Audit Logging** | Basic | CloudTrail/Custom |
| **Data Residency** | Platform-managed | Any AWS region |
| **Encryption at Rest** | ✓ | Configurable |
| **Custom Security Groups** | ✗ | ✓ |
| **WAF Integration** | ✗ | ✓ |

## Cost Analysis

### Convox Cloud Pricing

Simple, predictable monthly pricing:

| Machine Size | Monthly Cost | Included |
|--------------|--------------|----------|
| X-Small | $12* | 0.5 vCPU, 1 GB RAM, builds, SSL, monitoring |
| Small | $25 | 1 vCPU, 2 GB RAM, builds, SSL, monitoring |
| Medium | $75 | 2 vCPU, 4 GB RAM, builds, SSL, monitoring |
| Large | $150 | 4 vCPU, 8 GB RAM, builds, SSL, monitoring |

*500 free hours/month

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
| CloudWatch | ~$10 | Basic monitoring |
| **Total** | **~$228+** | Minimum production |

### Cost Comparison by Scale

| Workload | Convox Cloud | Self-Hosted Rack |
|----------|--------------|------------------|
| Dev/Test | $0-12 | ~$100 |
| Small Production | $25 | ~$228 |
| Medium Production | $50-100 | ~$400 |
| Large Production | $200-400 | ~$800+ |
| Enterprise | Contact Sales | $1,500+ |

## Performance Comparison

### Resource Allocation

**Convox Cloud**
- Fixed CPU:Memory ratios (1:2)
- Shared build infrastructure
- Network performance varies by machine size
- No dedicated resources below Large tier

**Self-Hosted Rack**
- Customizable ratios
- Dedicated build nodes optional
- Full network performance control
- All resources dedicated

### Scalability Limits

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

✓ **Rapid Deployment Priority**
- Need to deploy in minutes, not hours
- Prototype or MVP development
- Limited DevOps resources

✓ **Predictable Costs Required**
- Fixed monthly budgets
- Startups with cost constraints
- Projects with defined resource needs

✓ **Standard Architecture**
- Typical web applications
- Microservices under 10 services
- Standard networking requirements

✓ **Team Capabilities**
- Limited AWS expertise
- Small development teams
- Focus on application, not infrastructure

✓ **Business Requirements**
- SaaS applications
- B2B/B2C web applications
- Development and staging environments

### Choose Self-Hosted Rack When:

✓ **Full Control Required**
- Need Kubernetes access
- Custom operators or CRDs
- Complex networking requirements

✓ **Compliance Needs**
- HIPAA, PCI DSS requirements
- Data residency requirements
- Audit logging requirements

✓ **Advanced Architecture**
- Service mesh implementations
- Multi-region deployments
- Hybrid cloud setups

✓ **Resource Requirements**
- GPU workloads
- Memory-intensive applications (>8GB)
- Persistent storage needs

✓ **Integration Needs**
- VPC peering required
- Direct RDS/ElastiCache access
- IAM role integration

✓ **Scale Requirements**
- Very large applications
- Hundreds of services
- Custom autoscaling needs

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
- Cutting infrastructure costs
- Simplifying management

Migration effort: **Low-Medium**
- Remove unsupported features
- Adjust for limitations
- 2-4 hours typical

## Decision Matrix

| Factor | Weight | Convox Cloud | Self-Hosted Rack |
|--------|--------|--------------|------------------|
| **Setup Speed** | High | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |
| **Ease of Use** | High | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ |
| **Cost (Small)** | High | ⭐⭐⭐⭐⭐ | ⭐⭐ |
| **Cost (Large)** | Medium | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Flexibility** | Medium | ⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Control** | Low | ⭐ | ⭐⭐⭐⭐⭐ |
| **Compliance** | Varies | ⭐ | ⭐⭐⭐⭐⭐ |
| **Maintenance** | High | ⭐⭐⭐⭐⭐ | ⭐⭐ |
| **Scaling** | Medium | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| **Security** | High | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

## Hybrid Approach

Consider using both Cloud and Rack:

### Development on Cloud
- X-Small machines for developers
- Rapid iteration
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

## FAQ

### Can I migrate between Cloud and Rack?
Yes, applications are portable between Cloud and Rack with minor adjustments for Cloud limitations.

### Can I use both simultaneously?
Yes, many teams use Cloud for development/staging and Rack for production.

### Which is more secure?
Rack offers more security control, but Cloud provides managed security updates and patches.

### Which is faster to deploy?
Initial setup is faster on Cloud (minutes vs. 10-20 minutes), but deployment speeds are similar once configured.

### Can Cloud handle production workloads?
Yes, Cloud handles production workloads well within its resource limits (up to 4 vCPU, 8 GB RAM per machine).

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
# Create your application
$ convox cloud apps create my-app -i my-first-machine
# Then deploy
$ convox cloud deploy -i my-first-machine -a my-app
```

### Try Self-Hosted Rack
```bash
# Create Runtime Integration then Install Rack via Console at console.convox.com
# Create your application
$ convox apps create my-app -r my-first-rack
# Then deploy
$ convox deploy -r my-first-rack -a my-app
```

## Support and Resources

### Documentation
- [Cloud Getting Started](/cloud/getting-started)
- [Rack Installation](/getting-started/introduction)

### Contact
- Sales: sales@convox.com
- Support: cloud-support@convox.com
- Community: community.convox.com

## Conclusion

Both Convox Cloud and self-hosted Racks have their place:

- **Cloud** excels at simplicity, speed, and predictable costs
- **Rack** provides full control, flexibility, and enterprise features

Choose based on your team's needs, technical requirements, and growth trajectory. Start with Cloud for quick wins, migrate to Rack when you need more control.