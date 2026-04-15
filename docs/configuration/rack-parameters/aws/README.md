---
title: "AWS Rack Parameters"
slug: aws-rack-parameters
url: /configuration/rack-parameters/aws
---
# AWS Rack Parameters

The following parameters are available for configuring your Convox rack on Amazon Web Services (AWS). These parameters allow you to customize and optimize the behavior of your applications and services running on the AWS platform.

> Some parameters can only be set during rack installation and cannot be changed afterwards. These include `cidr`, `high_availability`, `private`, `private_subnets_ids`, `public_subnets_ids`, `vpc_id`, and `internet_gateway_id`. See individual parameter pages for details.

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
| [cert_duration](/configuration/rack-parameters/aws/cert_duration)                   | Specifies the certificate renewal period.                              |
| [cidr](/configuration/rack-parameters/aws/cidr)                                     | Specifies the CIDR range for the VPC.                                     |
| [convox_domain_tls_cert_disable](/configuration/rack-parameters/aws/convox_domain_tls_cert_disable) | Disables Convox domain TLS certificate generation for services. |
| [disable_convox_resolver](/configuration/rack-parameters/aws/disable_convox_resolver) | Disables the Convox resolver and uses the Kubernetes resolver instead. |
| [docker_hub_username](/configuration/rack-parameters/aws/docker_hub_username) | Configures Docker Hub username for authenticated image pulls (avoids rate limits). |
| [docker_hub_password](/configuration/rack-parameters/aws/docker_hub_password) | Sets Docker Hub access token for authenticated image pulls. Use with docker_hub_username. |
| [ebs_volume_encryption_enabled](/configuration/rack-parameters/aws/ebs_volume_encryption_enabled) | Enables encryption for EBS volumes used by primary node disks. |
| [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) | Enables ECR pull-through cache for Docker Hub images to avoid rate limits. |
| [ecr_scan_on_push_enable](/configuration/rack-parameters/aws/ecr_scan_on_push_enable) | Enables automatic vulnerability scanning for images pushed to ECR. |
| [efs_csi_driver_enable](/configuration/rack-parameters/aws/efs_csi_driver_enable)   | Enables the EFS CSI driver to use AWS EFS volumes.                       |
| [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable)               | Disables Fluentd installation in the rack.                               |
| [fluentd_memory](/configuration/rack-parameters/aws/fluentd_memory)                 | Configures memory allocation for the Fluentd log collector DaemonSet.    |
| [gpu_tag_enable](/configuration/rack-parameters/aws/gpu_tag_enable)                 | Enables GPU tagging.                                                     |
| [high_availability](/configuration/rack-parameters/aws/high_availability)           | Ensures high availability by creating a cluster with redundant resources. |
| [idle_timeout](/configuration/rack-parameters/aws/idle_timeout)                     | Specifies the idle timeout value for the Rack Load Balancer.             |
| [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens)             | Determines whether the Instance Metadata Service requires session tokens (IMDSv2). |
| [internal_router](/configuration/rack-parameters/aws/internal_router)               | Installs an internal load balancer within the VPC.                       |
| [internet_gateway_id](/configuration/rack-parameters/aws/internet_gateway_id)       | Specifies the ID of the attached internet gateway when using an existing VPC. |
| [karpenter_arch](/configuration/rack-parameters/aws/karpenter_arch)                 | Karpenter workload node CPU architecture. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_auth_mode](/configuration/rack-parameters/aws/karpenter_auth_mode)       | One-way migration preparing EKS for Karpenter. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_build_capacity_types](/configuration/rack-parameters/aws/karpenter_build_capacity_types) | Purchasing model for Karpenter build nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_build_consolidate_after](/configuration/rack-parameters/aws/karpenter_build_consolidate_after) | Delay before empty Karpenter build nodes are consolidated. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_build_cpu_limit](/configuration/rack-parameters/aws/karpenter_build_cpu_limit) | Maximum total vCPUs for the Karpenter build NodePool. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_build_instance_families](/configuration/rack-parameters/aws/karpenter_build_instance_families) | Instance families for Karpenter build nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_build_instance_sizes](/configuration/rack-parameters/aws/karpenter_build_instance_sizes) | Instance sizes for Karpenter build nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_build_memory_limit_gb](/configuration/rack-parameters/aws/karpenter_build_memory_limit_gb) | Maximum total memory for the Karpenter build NodePool. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_build_node_labels](/configuration/rack-parameters/aws/karpenter_build_node_labels) | Custom labels for Karpenter build nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_capacity_types](/configuration/rack-parameters/aws/karpenter_capacity_types) | EC2 purchasing model for Karpenter workload nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_config](/configuration/rack-parameters/aws/karpenter_config)             | JSON override for the Karpenter workload NodePool. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_consolidate_after](/configuration/rack-parameters/aws/karpenter_consolidate_after) | Delay before Karpenter consolidation triggers. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_consolidation_enabled](/configuration/rack-parameters/aws/karpenter_consolidation_enabled) | Enables Karpenter node consolidation. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_cpu_limit](/configuration/rack-parameters/aws/karpenter_cpu_limit)       | Maximum total vCPUs Karpenter can provision. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_disruption_budget_nodes](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes) | Maximum Karpenter nodes disrupted simultaneously. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled)           | Enables Karpenter node autoscaling. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_instance_families](/configuration/rack-parameters/aws/karpenter_instance_families) | EC2 instance families for Karpenter workload nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_instance_sizes](/configuration/rack-parameters/aws/karpenter_instance_sizes) | Instance sizes for Karpenter workload nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_memory_limit_gb](/configuration/rack-parameters/aws/karpenter_memory_limit_gb) | Maximum total memory Karpenter can provision. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_node_disk](/configuration/rack-parameters/aws/karpenter_node_disk)       | EBS volume size for Karpenter-provisioned nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_node_expiry)   | Maximum Karpenter node lifetime before replacement. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_node_labels](/configuration/rack-parameters/aws/karpenter_node_labels)   | Custom labels for Karpenter workload nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_node_taints](/configuration/rack-parameters/aws/karpenter_node_taints)   | Custom taints for Karpenter workload nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [karpenter_node_volume_type](/configuration/rack-parameters/aws/karpenter_node_volume_type) | EBS volume type for Karpenter-provisioned nodes. See [Karpenter](/configuration/scaling/karpenter). |
| [keda_enable](/configuration/rack-parameters/aws/keda_enable)                       | Enables KEDA (Kubernetes Event-Driven Autoscaling) for event-driven scaling. |
| [key_pair_name](/configuration/rack-parameters/aws/key_pair_name)                   | Specifies an EC2 Key Pair for SSH access to cluster nodes.               |
| [kubelet_registry_burst](/configuration/rack-parameters/aws/kubelet_registry_burst) | Sets the maximum burst rate for image pulls. See also [combined reference](/configuration/rack-parameters/aws/kubelet_registry_pull_params). |
| [kubelet_registry_pull_qps](/configuration/rack-parameters/aws/kubelet_registry_pull_qps) | Sets the steady-state rate limit for image pulls (queries per second). See also [combined reference](/configuration/rack-parameters/aws/kubelet_registry_pull_params). |
| [max_on_demand_count](/configuration/rack-parameters/aws/max_on_demand_count)       | Sets the maximum number of on-demand nodes when using the mixed capacity type. |
| [min_on_demand_count](/configuration/rack-parameters/aws/min_on_demand_count)       | Sets the minimum number of on-demand nodes when using the mixed capacity type. |
| [nlb_security_group](/configuration/rack-parameters/aws/nlb_security_group)         | Specifies the ID of the security group to attach to the NLB.             |
| [node_capacity_type](/configuration/rack-parameters/aws/node_capacity_type)         | Specifies the node capacity type: on-demand, spot, or mixed.             |
| [node_max_unavailable_percentage](/configuration/rack-parameters/aws/node_max_unavailable_percentage) | Controls the maximum percentage of nodes unavailable during node group updates. |
| [node_disk](/configuration/rack-parameters/aws/node_disk)                           | Specifies the node disk size in GB.                                      |
| [node_type](/configuration/rack-parameters/aws/node_type)                           | Specifies the node instance type.                                        |
| [nvidia_device_plugin_enable](/configuration/rack-parameters/aws/nvidia_device_plugin_enable) | Enables the NVIDIA GPU device plugin for GPU workloads. |
| [nvidia_device_time_slicing_replicas](/configuration/rack-parameters/aws/nvidia_device_time_slicing_replicas) | Configures GPU time slicing by setting the number of virtual replicas per physical GPU. |
| [pdb_default_min_available_percentage](/configuration/rack-parameters/aws/pdb_default_min_available_percentage) | Sets the default minimum percentage for Pod Disruption Budgets. |
| [pod_identity_agent_enable](/configuration/rack-parameters/aws/pod_identity_agent_enable) | Enables the AWS Pod Identity Agent. |
| [private](/configuration/rack-parameters/aws/private)                               | Specifies whether to place nodes in private subnets behind NAT gateways. |
| [private_subnets_ids](/configuration/rack-parameters/aws/private_subnets_ids)       | Specifies the IDs of private subnets to use for the Rack.                |
| [proxy_protocol](/configuration/rack-parameters/aws/proxy_protocol)                 | Enables the Proxy Protocol to track the original client IP address.      |
| [public_subnets_ids](/configuration/rack-parameters/aws/public_subnets_ids)         | Specifies the IDs of public subnets to use for the Rack.                 |
| [releases_to_retain_after_active](/configuration/rack-parameters/aws/releases_to_retain_after_active) | Specifies the number of releases to retain after the currently active release. |
| [releases_to_retain_task_run_interval_hour](/configuration/rack-parameters/aws/releases_to_retain_task_run_interval_hour) | Defines the interval in hours at which the release cleanup task runs. |
| [schedule_rack_scale_down](/configuration/rack-parameters/aws/schedule_rack_scale_down) | Specifies the schedule for scaling down the rack.                        |
| [schedule_rack_scale_up](/configuration/rack-parameters/aws/schedule_rack_scale_up) | Specifies the schedule for scaling up the rack.                          |
| [ssl_ciphers](/configuration/rack-parameters/aws/ssl_ciphers)                       | Specifies the SSL ciphers to use for Nginx.                              |
| [ssl_protocols](/configuration/rack-parameters/aws/ssl_protocols)                   | Specifies the SSL protocols to use for Nginx.                            |
| [syslog](/configuration/rack-parameters/aws/syslog)                                 | Specifies the endpoint to forward logs to a syslog server.               |
| [tags](/configuration/rack-parameters/aws/tags)                                     | Specifies custom tags to add to AWS resources.                           |
| [terraform_update_timeout](/configuration/rack-parameters/aws/terraform_update_timeout) | Controls how long Terraform waits for node group update operations to complete. |
| [user_data](/configuration/rack-parameters/aws/user_data)                           | Specifies custom commands to append to EC2 instance user data scripts.   |
| [user_data_url](/configuration/rack-parameters/aws/user_data_url)                   | Specifies a URL to a script to append to EC2 instance user data scripts. |
| [additional_karpenter_nodepools_config](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) | Creates custom Karpenter NodePools for specialized workloads. See [Karpenter](/configuration/scaling/karpenter). |
| [vpa_enable](/configuration/rack-parameters/aws/vpa_enable)                         | Enables the Vertical Pod Autoscaler (VPA) for automatic resource right-sizing. |
| [vpc_id](/configuration/rack-parameters/aws/vpc_id)                                 | Specifies the ID of an existing VPC to use for cluster creation.         |

## Setting Parameters

To set a rack parameter, use the following command:
```bash
$ convox rack params set parameterName=value -r rackName
Updating parameters... OK
```

For example, to set the `node_type` parameter:
```bash
$ convox rack params set node_type=m5.xlarge -r rackName
Updating parameters... OK
```

## Viewing Parameters

To view the current parameters for a rack:
```bash
$ convox rack params -r rackName
access_log_retention_in_days          7
build_node_enabled                    true
build_node_min_count                  0
build_node_type                       t3.small
```