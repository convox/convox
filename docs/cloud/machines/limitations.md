---
title: "Limitations and Differences"
draft: false
slug: limitations
url: /cloud/machines/limitations
---

# Limitations and Differences

While Convox Cloud provides the same application deployment capabilities as self-hosted Convox Racks, there are some limitations due to its managed, multi-tenant nature. This guide outlines these limitations and provides alternatives where applicable.

## Infrastructure Access Limitations

### No Direct Kubernetes Access

**Limitation**: You cannot use `kubectl` to directly interact with the underlying Kubernetes cluster.

**Impact**: 
- Cannot apply custom Kubernetes manifests
- Cannot install cluster-level operators or CRDs
- Cannot modify cluster configurations

**Alternatives**:
- Use `convox.yml` for all application configuration
- Request features through Convox support for platform-wide needs
- Consider self-hosted Rack if Kubernetes access is critical

### No SSH Access to Nodes

**Limitation**: Cannot SSH into the underlying EC2 instances or container nodes.

**Impact**:
- Cannot perform system-level debugging
- Cannot install system packages directly
- Cannot modify OS configurations

**Alternatives**:
- Use `convox cloud exec` for container access
- Include debugging tools in your Docker images
- Use comprehensive logging and monitoring

### Limited Shell Access

**Limitation**: Shell access is restricted to your application containers only.

**Available Commands**:
```bash
# Access running container
$ convox cloud exec my-service bash -a myapp -i my-machine

# Run one-off command
$ convox cloud run web "rails console" -a myapp -i my-machine
```

## Configuration Limitations

### Restricted Rack Parameters

Unlike self-hosted Racks, Cloud machines have limited configuration options:

**Not Available**:
- Node instance types (fixed to machine size)
- Network CIDR configuration
- Custom security groups
- VPC settings
- Autoscaling policies for nodes
- Custom IAM roles

**Available**:
- Machine size (X-Small, Small, Medium, Large)
- Region selection
- Application-level configurations via `convox.yml`

### Fixed Resource Ratios

**Limitation**: CPU to memory ratios are fixed per machine tier.

| Machine | vCPU:RAM Ratio |
|---------|----------------|
| X-Small | 1:2 GB |
| Small | 1:2 GB |
| Medium | 1:2 GB |
| Large | 1:2 GB |

**Impact**: Cannot customize for memory-heavy or CPU-heavy workloads independently.

**Alternatives**: Choose the machine size that best fits your primary constraint.

## Networking Limitations

### No Custom Load Balancers

**Limitation**: Cannot configure custom load balancer types or settings.

**Not Available**:
- Network Load Balancers (NLB)
- Custom ALB configurations
- Static IPs
- Custom SSL policies

**Available**:
- Standard HTTPS load balancing
- Automatic SSL via Let's Encrypt
- Custom domains
- Path-based routing via `convox.yml`

### Limited Network Policies

**Limitation**: Cannot define custom network policies or firewall rules.

**Impact**:
- Cannot restrict inter-service communication
- Cannot create network segments
- Limited to application-level security

**Alternatives**:
- Implement application-level authentication
- Use service mesh patterns in your application
- Leverage environment-based configuration

### No VPC Peering

**Limitation**: Cannot establish VPC peering or private connectivity to your own AWS resources.

**Impact**:
- Cannot directly connect to private RDS databases in your own AWS account
- Cannot access private S3 endpoints
- Cannot integrate with existing VPCs

**Alternatives**:
- Use Cloud Databases (managed RDS through Convox)
- Use public endpoints with security groups
- Implement API-based integration

## Database Resources

### Cloud Databases (Managed RDS)

Convox Cloud provides fully managed RDS databases through Cloud Databases:

**Available Database Types**:
- PostgreSQL (versions 14.x - 18.x)
- MySQL (versions 8.0.x and 8.4.x)
- MariaDB (versions 10.6.x - 11.8.x)

**Configuration Example**:
```yaml
resources:
  database:
    type: postgres
    provider: aws
    options:
      class: small
      version: 17.5
      durable: true

services:
  web:
    resources:
      - database
```

**Available Options**:
- Database class (dev, small, medium, large)
- Database version
- Multi-AZ failover (`durable: true`)

**Not Available**:
- Custom RDS instance types
- Custom storage sizes beyond class limits
- Read replicas
- Custom parameter groups

### Containerized Databases

You can also use containerized databases (without `provider: aws`):

```yaml
resources:
  database:
    type: postgres
    options:
      version: 13
      storage: 10
```

**Note**: Containerized databases do not persist data across machine restarts. Use Cloud Databases for production workloads.

### External Databases

**Not Available Through Cloud**:
- AWS ElastiCache
- DocumentDB
- DynamoDB

**Alternatives**:
- Use resource overlays to connect to external databases
- Provision managed databases separately and connect via environment variables

## Storage Limitations

### No Persistent Volumes

**Limitation**: No persistent volume support or EFS mounting.

**Impact**:
- Data not persisted across deployments
- Cannot share files between services
- Limited to ephemeral container storage

**Alternatives**:
- Use external object storage (S3)
- Store data in Cloud Databases
- Implement stateless architectures

### No Custom Volume Mounts

**Limitation**: Cannot mount custom volumes or host paths.

**Not Supported**:
```yaml
# This won't work in Cloud
volumes:
  - /host/path:/container/path
```

**Supported**:
- Container's ephemeral storage
- Volume sharing within a single service's containers

## Build Limitations

### Build Environment Restrictions

**Fixed Build Resources**:
- Build CPU: Shared pool (not configurable)
- Build Memory: 4 GB (not configurable)
- Build Timeout: 30 minutes (not configurable)
- Build Disk: 20 GB (not configurable)

**Impact**:
- Large builds may fail due to memory limits
- Cannot customize build performance
- Long builds may timeout

**Alternatives**:
- Optimize Dockerfiles for smaller builds
- Use multi-stage builds
- Pre-build base images

### No Custom Build Nodes

**Limitation**: Cannot provision dedicated or custom build infrastructure.

**Alternatives**:
- Use efficient build practices
- Leverage Docker layer caching
- Minimize build context size

## Security Limitations

### No Custom IAM Roles

**Limitation**: Cannot attach custom IAM roles to services.

**Impact**:
- Cannot directly access AWS services with IAM authentication
- Must use API keys for AWS service access

**Alternatives**:
- Use environment variables for credentials
- Implement service-to-service authentication
- Use temporary credentials with STS

### Limited Security Scanning

**Limitation**: No built-in container vulnerability scanning.

**Alternatives**:
- Scan images in your CI/CD pipeline
- Use external scanning services
- Regularly update base images

## Service Limitations

### No Custom Services

**Limitation**: Cannot deploy system-level services or operators.

**Not Supported**:
- Kubernetes operators
- Admission webhooks  
- Custom controllers
- Cluster-wide services
- DaemonSets

**Supported**:
- Standard application services via `convox.yml`
- Sidecar containers within your services
- Init containers

### Agent Services Not Supported

**Limitation**: Cannot deploy agent-type services that run on every node.

**Impact**:
- Cannot deploy custom monitoring agents
- Cannot run node-level services
- Cannot install custom log collectors

**Alternatives**:
- Use built-in monitoring and logging
- Include agents in your application containers
- Use external SaaS monitoring solutions

## Scaling Limitations

### Machine-Level Constraints

**Limitation**: Services cannot scale beyond machine boundaries.

**Maximum Resources per Machine**:
| Machine | Max Total CPU | Max Total RAM |
|---------|---------------|---------------|
| X-Small | 500m | 1 GB |
| Small | 1000m | 2 GB |
| Medium | 2000m | 4 GB |
| Large | 4000m | 8 GB |

**Impact**: 
- Single service cannot exceed machine resources
- All services combined cannot exceed machine limits

**Alternatives**:
- Upgrade to larger machine size
- Distribute services across multiple machines
- Optimize resource usage

### No Cross-Machine Scaling

**Limitation**: A single application cannot automatically scale across multiple machines.

**Alternatives**:
- Deploy to larger machines
- Split into microservices on separate machines
- Implement application-level sharding

## Monitoring and Debugging Limitations

### Basic Monitoring Only

**Available Metrics**:
- CPU usage (basic)
- Memory usage (basic)
- Request counts
- Error rates (HTTP)

**Not Available**:
- Custom metrics
- APM integration
- Distributed tracing
- Custom dashboards

**Alternatives**:
- Export metrics from your application
- Use external monitoring services
- Implement application-level monitoring

### Limited Log Retention

**Limitation**: Logs retained for 7 days only.

**Alternatives**:
- Stream logs to external service
- Implement application-level log shipping
- Use external log aggregation

## Compliance and Certification

### Compliance Limitations

**Not Available**:
- HIPAA compliance options
- PCI DSS certification
- SOC 2 attestation
- ISO certifications
- FedRAMP authorization

**Available**:
- Standard security practices
- Encrypted data in transit
- Regular security updates

**Alternatives**:
- Self-hosted Rack for compliance needs
- Implement application-level compliance measures
- Contact sales for enterprise options

## Workarounds and Best Practices

### Adapting to Limitations

1. **Design for Statelessness**
   - Store state in Cloud Databases
   - Use databases for persistence
   - Implement session storage in Redis

2. **Optimize Resource Usage**
   ```yaml
   services:
     web:
       scale:
         cpu: 250
         memory: 512
   ```

3. **Use Cloud Databases for Production**
   ```yaml
   resources:
     database:
       type: postgres
       provider: aws
       options:
         class: small
         version: 17.5
         durable: true
   ```

4. **Implement at Application Level**
   - Security: Application-level authentication
   - Networking: Service mesh patterns
   - Monitoring: APM libraries

### When to Consider Self-Hosted Rack

Consider a self-hosted Convox Rack if you need:

- Direct Kubernetes access
- Custom infrastructure configuration
- Compliance certifications
- Private networking/VPC peering
- Custom IAM roles
- Persistent storage
- System-level customization
- Dedicated resources
- Advanced RDS features (read replicas, custom instance types)

## Feature Comparison Table

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| Kubernetes Access | ✗ | ✓ |
| SSH to Nodes | ✗ | ✓ |
| Custom Node Types | ✗ | ✓ |
| VPC Configuration | ✗ | ✓ |
| Network Policies | ✗ | ✓ |
| Persistent Volumes | ✗ | ✓ |
| Custom IAM Roles | ✗ | ✓ |
| Managed Databases | ✓ (Cloud DBs) | ✓ (RDS Resources) |
| RDS Read Replicas | ✗ | ✓ |
| Agent Services | ✗ | ✓ |
| Custom Build Nodes | ✗ | ✓ |
| Rack Parameters | Limited | Full |
| Setup Time | Instant | 10-20 min |
| Maintenance | None | Required |
| Pricing | Per Machine/DB | Infrastructure |

## Getting Help

If you're unsure whether Convox Cloud meets your requirements:

1. Review your application architecture against these limitations
2. Contact support@convox.com with specific requirements
3. Try the X-Small tier to test compatibility
4. Consider self-hosted Rack for full control

## Next Steps

- [CLI Reference](/cloud/cli-reference) - Learn available commands
- [Cloud Databases](/cloud/databases) - Database configuration options
- [Migration Guide](/cloud/migration-guide) - Move from other platforms
- [Comparison](/cloud/comparison) - Detailed Cloud vs Rack comparison