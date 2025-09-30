---
title: "Convox Cloud"
draft: false
slug: cloud
url: /cloud
---

# Convox Cloud

Convox Cloud provides the power and flexibility of Convox without the complexity of managing your own infrastructure. Built on the same reliable technology as Convox Rack, Cloud offers a fully-managed, multi-tenant platform with predictable pricing and instant deployment capabilities.

## What is Convox Cloud?

Convox Cloud introduces **Machines** - dedicated compute resources that host your applications in a managed environment. Unlike traditional Convox Racks that run in your own cloud account, Machines run on Convox-managed infrastructure, eliminating the need for cloud provider setup, Kubernetes management, or infrastructure maintenance.

### Key Benefits

- **Zero Infrastructure Management**: No AWS account, no Kubernetes cluster, no maintenance overhead
- **Instant Setup**: Deploy applications in seconds, not minutes
- **Predictable Pricing**: Simple per-machine pricing with no hidden infrastructure costs
- **Full Convox Compatibility**: Use the same `convox.yml` configuration and deployment workflows
- **Automatic Scaling**: Built-in autoscaling based on your application needs
- **Isolated Builds**: Dedicated build environments ensure reliable deployments

## Cloud vs Self-Hosted Racks

| Feature | Convox Cloud | Self-Hosted Rack |
|---------|--------------|------------------|
| **Setup Time** | Instant | 10-20 minutes |
| **Cloud Account Required** | No | Yes |
| **Infrastructure Management** | Convox-managed | Self-managed |
| **Pricing Model** | Per machine/month | Infrastructure costs |
| **Kubernetes Access** | Restricted | Full access |
| **Customization** | Limited | Full control |
| **Best For** | Rapid deployment, predictable costs | Complete control, custom requirements |

## Machine Concept

A Machine is a slice of dedicated compute resources (vCPU and RAM) that hosts your applications. Machines come in predefined sizes to match your workload requirements:

- **X-Small**: 0.5 vCPU, 1 GB RAM - Perfect for development and testing
- **Small**: 1 vCPU, 2 GB RAM - Ideal for production applications
- **Medium**: 2 vCPU, 4 GB RAM - For growing applications
- **Large**: 4 vCPU, 8 GB RAM - For high-traffic production workloads

Each machine can host multiple services as long as their combined resource usage doesn't exceed the machine's capacity.

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
You can get your login token from the Account page in the [Convox Console](https://console.convox.com) or during the guided onboarding flow.

3. **Create a Machine**:
   - Log into the [Convox Console](https://console.convox.com)
   - Navigate to the Cloud Machines page
   - Click the "New Machine" button
   - Select your desired size and region
   - Click "Create Machine"

4. **Deploy an Application**:
```bash
# Create your application
$ convox cloud apps create my-app -i my-machine

# Deploy to the machine
$ convox cloud deploy -i my-machine -a my-app
```

## Core Concepts

### Machines
Pre-configured compute units that provide CPU, memory, and storage for your applications. Machines are fully managed and automatically maintained by Convox.

### Applications
Standard Convox applications that run on machines. Applications use the same `convox.yml` format and support the same features as rack-based deployments.

### Build Isolation
Builds run in separate, ephemeral environments to prevent resource contention and ensure consistent, reliable deployments.

### Automatic Updates
System components and security patches are automatically applied by Convox, keeping your applications secure without maintenance windows.

## Use Cases

Convox Cloud is ideal for:

- **Startups**: Get to market quickly without infrastructure overhead
- **Development Teams**: Focus on building features, not managing servers
- **Agencies**: Deploy client applications with predictable pricing
- **Side Projects**: Cost-effective hosting with professional features
- **Staging Environments**: Quickly spin up review apps and staging environments

## Next Steps

- [Get Started with Convox Cloud](/cloud/getting-started) - Step-by-step guide to deploying your first application
- [Machine Management](/cloud/machines) - Learn how to create and manage machines
- [CLI Reference](/cloud/cli-reference) - Complete reference for `convox cloud` commands
- [Migration Guide](/cloud/migration-guide) - Migrate from other platforms to Convox Cloud

## Support

For questions about Convox Cloud:

- Documentation: [docs.convox.com](https://docs.convox.com)
- Community: [community.convox.com](https://community.convox.com)
- Support: cloud-support@convox.com