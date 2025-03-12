---
title: "AWS Rack Parameters"
draft: false
slug: aws-rack-parameters
url: /configuration/rack-parameters/aws
---
# AWS Rack Parameters

The following parameters are available for configuring your Convox rack on Amazon Web Services (AWS). These parameters allow you to customize and optimize the behavior of your applications and services running on the AWS platform.

## Parameters

| Parameter                            | Description                                                              |
|:-------------------------------------|:-------------------------------------------------------------------------|
| [access_log_retention_in_days](/configuration/rack-parameters/aws/access_log_retention_in_days) | Specifies the retention period for Nginx access logs stored in CloudWatch Logs. |
| [additional_build_groups_config](/configuration/rack-parameters/aws/additional_build_groups_config) | Defines dedicated node groups specifically for application build processes. |
| [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config) | Configures additional customized node groups for the cluster. |
| [availability_zones](/configuration/rack-parameters/aws/availability_zones)         | Specifies a list of Availability Zones for better availability and fault tolerance. |
| [build_disable_convox_resolver](/configuration/rack-parameters/aws/build_disable_convox_resolver) | Disables the Convox DNS resolver during builds to address DNS resolution issues. |
| [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled)         | Enables a dedicated build node for building applications.                |
| [build_node_min_count](/configuration/rack-parameters/aws/build_node_min_count)     | Sets the minimum number of build nodes to keep running.                  |
| [build_node_type](/configuration/rack-parameters/aws/build_node_type)               | Specifies the node type for the build node.                              |
| [cert_duration](/configuration/rack-parameters/aws/cert_duration)                   | Specifies the certification renewal period.                              |
| [cidr](/configuration/rack-parameters/aws/cidr)                                     | Specifies the CIDR range for the VPC.                                     |
| [convox_domain_tls_cert_disable](/configuration/rack-parameters/aws/convox_domain_tls_cert_disable) | Disables Convox domain TLS certificate generation for services. |
| [ecr_scan_on_push_enable](/configuration/rack-parameters/aws/ecr_scan_on_push_enable) | Enables automatic vulnerability scanning for images pushed to ECR. |
| [efs_csi_driver_enable](/configuration/rack-parameters/aws/efs_csi_driver_enable)   | Enables the EFS CSI driver to use AWS EFS volumes.                       |
| [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable)               | Disables Fluentd installation in the rack.                               |
| [gpu_tag_enable](/configuration/rack-parameters/aws/gpu_tag_enable)                 | Enables GPU tagging.                                                     |
| [high_availability](/configuration/rack-parameters/aws/high_availability)           | Ensures high availability by creating a cluster with redundant resources. |
| [idle_timeout](/configuration/rack-parameters/aws/idle_timeout)                     | Specifies the idle timeout value for the Rack Load Balancer.             |
| [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens)             | Determines whether the Instance Metadata Service requires session tokens (IMDSv2). |
| [internal_router](/configuration/rack-parameters/aws/internal_router)               | Installs an internal load balancer within the VPC.                       |
| [internet_gateway_id](/configuration/rack-parameters/aws/internet_gateway_id)       | Specifies the ID of the attached internet gateway when using an existing VPC. |
| [kubelet_registry_burst](/configuration/rack-parameters/aws/kubelet_registry_pull_params) | Sets the maximum burst rate for image pulls. |
| [kubelet_registry_pull_qps](/configuration/rack-parameters/aws/kubelet_registry_pull_params) | Sets the steady-state rate limit for image pulls (queries per second). |
| [max_on_demand_count](/configuration/rack-parameters/aws/max_on_demand_count)       | Sets the maximum number of on-demand nodes when using the mixed capacity type. |
| [min_on_demand_count](/configuration/rack-parameters/aws/min_on_demand_count)       | Sets the minimum number of on-demand nodes when using the mixed capacity type. |
| [nlb_security_group](/configuration/rack-parameters/aws/nlb_security_group)         | Specifies the ID of the security group to attach to the NLB.             |
| [node_capacity_type](/configuration/rack-parameters/aws/node_capacity_type)         | Specifies the node capacity type: on-demand, spot, or mixed.             |
| [node_disk](/configuration/rack-parameters/aws/node_disk)                           | Specifies the node disk size in GB.                                      |
| [node_type](/configuration/rack-parameters/aws/node_type)                           | Specifies the node instance type.                                        |
| [pdb_default_min_available_percentage](/configuration/rack-parameters/aws/pdb_default_min_available_percentage) | Sets the default minimum percentage for Pod Disruption Budgets. |
| [pod_identity_agent_enable](/configuration/rack-parameters/aws/pod_identity_agent_enable) | Enables the AWS Pod Identity Agent. |
| [private](/configuration/rack-parameters/aws/private)                               | Specifies whether to place nodes in private subnets behind NAT gateways. |
| [private_subnets_ids](/configuration/rack-parameters/aws/private_subnets_ids)       | Specifies the IDs of private subnets to use for the Rack.                |
| [proxy_protocol](/configuration/rack-parameters/aws/proxy_protocol)                 | Enables the Proxy Protocol to track the original client IP address.      |
| [public_subnets_ids](/configuration/rack-parameters/aws/public_subnets_ids)         | Specifies the IDs of public subnets to use for the Rack.                 |
| [schedule_rack_scale_down](/configuration/rack-parameters/aws/schedule_rack_scale_down) | Specifies the schedule for scaling down the rack.                        |
| [schedule_rack_scale_up](/configuration/rack-parameters/aws/schedule_rack_scale_up) | Specifies the schedule for scaling up the rack.                          |
| [ssl_ciphers](/configuration/rack-parameters/aws/ssl_ciphers)                       | Specifies the SSL ciphers to use for Nginx.                              |
| [ssl_protocols](/configuration/rack-parameters/aws/ssl_protocols)                   | Specifies the SSL protocols to use for Nginx.                            |
| [syslog](/configuration/rack-parameters/aws/syslog)                                 | Specifies the endpoint to forward logs to a syslog server.               |
| [tags](/configuration/rack-parameters/aws/tags)                                     | Specifies custom tags to add to AWS resources.                           |
| [user_data](/configuration/rack-parameters/aws/user_data)                           | Specifies custom commands to append to EC2 instance user data scripts.   |
| [user_data_url](/configuration/rack-parameters/aws/user_data_url)                   | Specifies a URL to a script to append to EC2 instance user data scripts. |
| [vpc_id](/configuration/rack-parameters/aws/vpc_id)                                 | Specifies the ID of an existing VPC to use for cluster creation.         |
