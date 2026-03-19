---
title: "Azure Rack Parameters"
slug: azure-rack-parameters
url: /configuration/rack-parameters/azure
---
# Azure Rack Parameters

The following parameters are available for configuring your Convox rack on Microsoft Azure. These parameters allow you to customize and optimize the behavior of your applications and services running on the Azure platform.

> Some parameters can only be set during rack installation and cannot be changed afterwards. These include `region`. See individual parameter pages for details.

## Parameters

| Parameter                            | Description                                                              |
|:-------------------------------------|:-------------------------------------------------------------------------|
| [additional_build_groups_config](/configuration/rack-parameters/azure/additional_build_groups_config) | Configures additional dedicated build node pools for the cluster. |
| [additional_node_groups_config](/configuration/rack-parameters/azure/additional_node_groups_config) | Configures additional customized node pools for the cluster. |
| [cert_duration](/configuration/rack-parameters/azure/cert_duration) | Certification renew period. |
| [docker_hub_password](/configuration/rack-parameters/azure/docker_hub_password) | Docker Hub access token for authenticated image pulls. |
| [docker_hub_username](/configuration/rack-parameters/azure/docker_hub_username) | Docker Hub username for authenticated image pulls. |
| [high_availability](/configuration/rack-parameters/azure/high_availability) | Enables high availability mode with redundant replicas. |
| [idle_timeout](/configuration/rack-parameters/azure/idle_timeout) | Load balancer idle timeout in minutes. |
| [max_on_demand_count](/configuration/rack-parameters/azure/max_on_demand_count) | Maximum number of nodes in the default node pool. |
| [min_on_demand_count](/configuration/rack-parameters/azure/min_on_demand_count) | Minimum number of nodes in the default node pool. |
| [nginx_additional_config](/configuration/rack-parameters/azure/nginx_additional_config) | Additional nginx ingress controller configuration. |
| [nginx_image](/configuration/rack-parameters/azure/nginx_image) | Custom nginx ingress controller image. |
| [node_disk](/configuration/rack-parameters/azure/node_disk) | OS disk size in GB for the default node pool. |
| [node_type](/configuration/rack-parameters/azure/node_type) | Specifies the node instance type. |
| [nvidia_device_plugin_enable](/configuration/rack-parameters/azure/nvidia_device_plugin_enable) | Deploys the NVIDIA GPU Device Plugin to enable GPU workloads. |
| [nvidia_device_time_slicing_replicas](/configuration/rack-parameters/azure/nvidia_device_time_slicing_replicas) | Number of virtual GPU replicas per physical GPU for time-slicing. |
| [pdb_default_min_available_percentage](/configuration/rack-parameters/azure/pdb_default_min_available_percentage) | Default minimum available percentage for PDBs. |
| [region](/configuration/rack-parameters/azure/region) | Specifies the Azure region for the rack. |
| [ssl_ciphers](/configuration/rack-parameters/azure/ssl_ciphers) | Custom SSL/TLS cipher suites for nginx. |
| [ssl_protocols](/configuration/rack-parameters/azure/ssl_protocols) | SSL/TLS protocol versions for nginx. |
| [syslog](/configuration/rack-parameters/azure/syslog) | Specifies the endpoint to forward logs to a syslog server. |
| [tags](/configuration/rack-parameters/azure/tags) | Custom Azure resource tags. |
