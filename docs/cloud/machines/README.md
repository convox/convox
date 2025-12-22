---
title: "Machine Management"
draft: false
slug: machines
url: /cloud/machines
---

# Machine Management

Machines are the fundamental compute units in Convox Cloud. Each machine provides dedicated vCPU and memory resources for running your applications in a fully-managed environment.

## What is a Machine?

A Machine is a slice of compute resources (vCPU and RAM) that hosts your Convox applications. Unlike traditional Convox Racks that require you to manage your own cloud infrastructure, Machines run on Convox-managed infrastructure with:

- Pre-configured Kubernetes environment
- Automatic security updates
- Built-in load balancing
- Managed SSL certificates
- Isolated build environments

## Creating Machines

### Using the Console

To create a machine:

1. Log into the [Convox Console](https://console.convox.com)
2. Navigate to the Cloud Machines page
3. Click the "New Machine" button
4. Configure your machine:
   - **Name**: Choose a descriptive name
   - **Size**: Select from X-Small, Small, Medium, or Large
   - **Region**: Choose your AWS region (us-east-1, us-west-2, eu-west-1, etc.)
5. Click "Create Machine"

The machine will be provisioned and ready for use within a few moments.

## Listing Machines

View all your machines using the CLI:

```bash
$ convox cloud machines
NAME                          CPU   MEMORY  AGE
mycompany/production-api      4000  8192    2w3d
mycompany/production-db       4000  8192    1mo2d
mycompany/staging-backend     2000  4096    5d12h
mycompany/staging-workers     1000  2048    3d8h
mycompany/dev-environment     500   1024    12h45m
mycompany/feature-testing     500   1024    2h30m
```

## Machine Sizes

Machines come in four pre-configured sizes:

| Size | vCPU | RAM | Monthly Price | Best For |
|------|------|-----|---------------|----------|
| **X-Small** | 0.5 | 1 GB | $12 | Development, testing, low-traffic apps |
| **Small** | 1 | 2 GB | $25 | Standard production applications |
| **Medium** | 2 | 4 GB | $75 | Growing applications, multiple services |
| **Large** | 4 | 8 GB | $150 | High-traffic production, resource-intensive apps |

## Upgrading and Downgrading Machines

Machine resize functionality is coming soon. In the meantime, to change your machine size:

1. Create a new machine with your desired size through the [Convox Console](https://console.convox.com)
2. Deploy your application to the new machine
3. Verify your application is running correctly on the new machine
4. Remove the old machine once you've confirmed everything is working

This ensures zero downtime during the transition.

## Resource Allocation

### Understanding Resource Limits

Each machine has fixed CPU and memory limits that are shared among all services running on it. When deploying applications:

1. The sum of all service resource requests cannot exceed the machine's capacity
2. Services can burst above their requests if unused capacity is available
3. Services will be throttled if they exceed the machine's total capacity

### Example Multi-Service Configuration

For a Small machine (1 vCPU, 2 GB RAM):

```yaml
services:
  web:
    build: .
    port: 3000
    scale:
      count: 2
      cpu: 250
      memory: 512
  
  worker:
    build: .
    command: npm run worker
    scale:
      count: 1
      cpu: 250
      memory: 768
  
  cache:
    image: redis:alpine
    port: 6379
    scale:
      cpu: 125
      memory: 256
```

Total resource usage:
- CPU: (2 × 250) + 250 + 125 = 875 millicores (under 1000 limit)
- Memory: (2 × 512) + 768 + 256 = 2048 MB (at 2 GB limit)

## Cloud Databases

Convox Cloud provides managed database instances separately from machines. Cloud Databases are billed independently and include:

- PostgreSQL, MySQL, and MariaDB
- 7-day automated backups
- Optional Multi-AZ failover

See [Cloud Databases](/cloud/databases) for configuration details.

### Example with Database

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
    resources:
      - database
```

## Managing Applications on Machines

### Deploying Applications

Deploy an application to a specific machine:

```bash
$ convox cloud deploy -i my-machine
```

### Listing Applications

View all applications on a machine:

```bash
$ convox cloud apps -i production-api
APP       STATUS   RELEASE
web-app   running  RWIUFPDLJFV
api       running  RPHAPCORTEQ
worker    running  RDYSBVNHLNG
backend   running  RLFFBKELARR
service   running  RPHAPCORTEQ
```

### Moving Applications Between Machines

To move an application to a different machine:

1. Create the application on the new machine:
```bash
$ convox cloud apps create my-app -i new-machine
```

2. Deploy to the new machine:
```bash
$ convox cloud deploy -i new-machine -a my-app
```

3. Verify the application is running:
```bash
$ convox cloud services -a my-app -i new-machine
```

4. Remove from the old machine (optional):
```bash
$ convox cloud apps delete my-app -i old-machine
```

## Machine and App Configuration

### Environment Variables

Set application-specific environment variables:

```bash
$ convox cloud env set SECRET_KEY=$(openssl rand -hex 64) -a my-app -i my-machine
Setting SECRET_KEY... OK
Release: RUAPYWIYWIO
```

### Process Monitoring

View running processes for an application:

```bash
$ convox cloud ps -a my-app -i production-api
ID                    SERVICE  STATUS   RELEASE      STARTED       COMMAND
app-859886fd8d-f6lvd  web      running  RWIUFPDLJFV  10 hours ago
app-859886fd8d-g7mwe  worker   running  RWIUFPDLJFV  10 hours ago
app-859886fd8d-h8nxf  web      running  RWIUFPDLJFV  10 hours ago
```

## Build Configuration

Machines include isolated build environments that don't consume machine resources:

- Builds run in ephemeral containers
- Build cache is maintained per machine
- Parallel builds are queued to prevent resource conflicts

## Machine Limits

Each machine has the following operational limits:

| Limit | Value | Notes |
|-------|-------|-------|
| Max services per machine | 20 | Total across all applications |
| Max processes per service | 10 | Can be increased with autoscaling |
| Max applications | 10 | Per machine |
| Build timeout | 30 minutes | Per build |
| Max environment variables | 100 | Per application |

## Machine Deletion

To delete a machine:

1. Log into the [Convox Console](https://console.convox.com)
2. Navigate to the Cloud Machines page
3. Select the machine you want to delete
4. Click "Delete Machine"
5. Confirm the deletion

**Warning**: Deleting a machine will:
- Remove all applications running on it
- Delete all associated data
- Cannot be undone

Note: Cloud Databases are managed separately and are not deleted when a machine is deleted.

## Best Practices

### Right-Sizing Machines

1. **Start small**: Begin with an X-Small or Small machine
2. **Monitor usage**: Track CPU and memory utilization
3. **Scale gradually**: Upgrade only when consistently near limits
4. **Use autoscaling**: Let services scale within machine limits

### Resource Planning

1. **Leave headroom**: Keep 20-30% resources free for bursts
2. **Balance services**: Distribute load across multiple small services
3. **Optimize builds**: Use multi-stage Docker builds to reduce image size
4. **Cache dependencies**: Leverage Docker layer caching

### Cost Optimization

1. **Consolidate services**: Run related services on the same machine
2. **Use appropriate sizes**: Don't over-provision for small workloads
3. **Schedule scaling**: Use autoscaling for variable loads
4. **Regular review**: Audit machine usage monthly

## Troubleshooting

### Machine Won't Create

- Verify your account has an attached payment method
- Ensure valid machine name (lowercase, alphanumeric, hyphens)
- Contact us at cloud-support@convox.com

### Application Won't Deploy

- Check machine has sufficient resources
- Verify application fits within limits
- Review build logs for errors
- Review app service logs for errors

### Performance Issues

- Monitor CPU and memory usage
- Consider upgrading machine size
- Optimize application resource requests
- Enable autoscaling for variable loads

## Next Steps

- [Sizing and Pricing](/cloud/machines/sizing-and-pricing) - Detailed pricing information
- [Cloud Databases](/cloud/databases) - Database configuration options
- [Limitations](/cloud/machines/limitations) - Understand machine constraints
- [CLI Reference](/cloud/cli-reference) - Complete command documentation