---
title: "Azure Rack Parameters"
draft: false
slug: azure-rack-parameters
url: /configuration/rack-parameters/azure
---
# Azure Rack Parameters

The following parameters are available for configuring your Convox rack on Microsoft Azure. These parameters allow you to customize and optimize the behavior of your applications and services running on the Azure platform.

## Parameters

| Parameter                            | Description                                                              |
|:-------------------------------------|:-------------------------------------------------------------------------|
| [cert_duration](/configuration/rack-parameters/azure/cert_duration)                                             | Certification renew period.                                               |
| [node_type](/configuration/rack-parameters/azure/node_type)                                                     | Specifies the node instance type.                                         |
| [nvidia_device_plugin_enable](/configuration/rack-parameters/azure/nvidia_device_plugin_enable)                 | Deploys the NVIDIA GPU Device Plugin to enable GPU workloads.             |
| [nvidia_device_time_slicing_replicas](/configuration/rack-parameters/azure/nvidia_device_time_slicing_replicas) | Number of virtual GPU replicas per physical GPU for time-slicing.         |
| [region](/configuration/rack-parameters/azure/region)                                                           | Specifies the Azure region for the rack.                                  |
| [syslog](/configuration/rack-parameters/azure/syslog)                                                           | Specifies the endpoint to forward logs to a syslog server.                |
| [high_availability](/configuration/rack-parameters/azure/high_availability)                               | Enables high availability mode with redundant replicas.                   |
| [idle_timeout](/configuration/rack-parameters/azure/idle_timeout)                                         | Load balancer idle timeout in minutes.                                    |
| [internal_router](/configuration/rack-parameters/azure/internal_router)                                   | Enables an internal load balancer for private traffic.                    |
| [nginx_additional_config](/configuration/rack-parameters/azure/nginx_additional_config)                   | Additional nginx ingress controller configuration.                        |
| [nginx_image](/configuration/rack-parameters/azure/nginx_image)                                           | Custom nginx ingress controller image.                                    |
| [node_type](/configuration/rack-parameters/azure/node_type)                                               | Specifies the node instance type.                                         |
| [pdb_default_min_available_percentage](/configuration/rack-parameters/azure/pdb_default_min_available_percentage) | Default minimum available percentage for PDBs.                      |
| [proxy_protocol](/configuration/rack-parameters/azure/proxy_protocol)                                     | Enables PROXY protocol on the nginx ingress controller.                   |
| [ssl_ciphers](/configuration/rack-parameters/azure/ssl_ciphers)                                           | Custom SSL/TLS cipher suites for nginx.                                   |
| [ssl_protocols](/configuration/rack-parameters/azure/ssl_protocols)                                       | SSL/TLS protocol versions for nginx.                                      |
| [tags](/configuration/rack-parameters/azure/tags)                                                         | Custom Azure resource tags.                                               |
