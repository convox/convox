---
title: "Rack Parameters"
draft: false
slug: rack-parameters
url: /configuration/rack-parameters
---
# Rack Parameters

Rack parameters are configuration settings that control various aspects of your Convox rack. These parameters allow you to customize and optimize the behavior of your applications and services running on the rack.

## Installing Racks with Parameters

When installing a new rack through the Convox Console, you can configure parameters at installation time. This allows you to customize your rack configuration from the start, rather than modifying parameters after installation.

### Using the Install Modal

During rack installation, you can set any available parameters directly in the install modal. Parameters not explicitly set will use their default values.

### Parameter Templates

The Convox Console provides pre-created parameter templates to help you quickly configure racks for common use cases:

- **Production**: Optimized for production workloads with enhanced security and reliability
- **Staging**: Balanced configuration suitable for staging environments
- **Minimalist**: Minimal resource configuration for cost-sensitive deployments
- **CMS**: Optimized for content management systems
- **Default**: Basic configuration with most features disabled to avoid interference with existing infrastructure

### Using JSON Parameter Files

You can upload a JSON file containing your desired parameter configurations. This feature allows you to:
- Quickly replicate rack configurations across environments
- Store and version control your infrastructure settings
- Share standardized configurations with your team

Here's an example JSON parameter file:

```json
{
    "node_type": "t3.medium",
    "node_capacity_type": "on_demand",
    "build_node_enabled": "true",
    "build_node_type": "t3.large",
    "high_availability": "true",
    "node_disk": "20",
    "min_on_demand_count": "2",
    "max_on_demand_count": "10",
    "proxy_protocol": "true",
    "efs_csi_driver_enable": "true"
}
```

This example configuration:
- Uses `t3.medium` instances for regular nodes with on-demand capacity
- Enables a dedicated build node using `t3.large` instances
- Ensures high availability with a minimum of 2 on-demand nodes
- Increases node disk size to 20GB
- Enables proxy protocol for client IP preservation
- Enables EFS CSI driver for shared storage capabilities

**Note**: You only need to include parameters you want to change from their defaults. Any parameters not specified in your JSON file will use their default values.

### Best Practices for Parameter Templates

1. **Start Small**: Begin with minimal changes and add parameters as needed
2. **Document Your Choices**: Comment your JSON files to explain why specific parameters were chosen
3. **Test First**: Always test parameter configurations in non-production environments first
4. **Version Control**: Store your parameter JSON files in version control alongside your application code
5. **Environment-Specific**: Create different parameter files for different environments (dev, staging, production)

## Managing Rack Parameters

### Viewing Current Parameters
To view the current rack parameters, use the following command:
```html
$ convox rack params -r rackName
```
This command displays the current values of all rack parameters for the specified rack.

### Setting Parameters
To set a rack parameter, use the following command:
```html
$ convox rack params set parameterName=value -r rackName
Setting parameters... OK
```
This command sets the specified parameter to the given value.

## Cloud Providers

- [Amazon Web Services (AWS)](/configuration/rack-parameters/aws)
- [Google Cloud Platform (GCP)](/configuration/rack-parameters/gcp)
- [Microsoft Azure](/configuration/rack-parameters/azure)
- [Digital Ocean](/configuration/rack-parameters/do)

Select your cloud provider to view the available parameters and their configurations.