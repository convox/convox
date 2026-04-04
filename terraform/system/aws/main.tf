provider "aws" {
  region = var.region
}

provider "kubernetes" {
  cluster_ca_certificate = var.private_eks_host != "" ? null : module.cluster.ca
  host                   = var.private_eks_host != "" ? var.private_eks_host : module.cluster.endpoint

  dynamic "exec" {
    for_each = var.private_eks_host != "" ? [] : [1]
    content {
      api_version = "client.authentication.k8s.io/v1beta1"
      args        = ["eks", "get-token", "--cluster-name", local.name]
      command     = "aws"
    }
  }

  insecure = var.private_eks_host != "" ? true : false
  username = var.private_eks_host != "" ? var.private_eks_user : null
  password = var.private_eks_host != "" ? var.private_eks_pass : null
}

data "aws_eks_cluster_auth" "cluster" {
  name = module.cluster.id
}

data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
  request_headers = {
    User-Agent = "convox"
  }
}

locals {
  name      = lower(var.name)
  rack_name = lower(var.rack_name)

  // var.node_type can be assigned a comma separated list of instance types
  node_type       = split(",", var.node_type)[0]
  build_node_type = var.build_node_type != "" ? var.build_node_type : local.node_type
  arm_type        = module.node_arch.is_arm
  build_arm_type  = module.build_node_arch.is_arm
  current         = jsondecode(data.http.releases.response_body).tag_name
  gpu_type        = substr(local.node_type, 0, 1) == "g" || substr(local.node_type, 0, 1) == "p"
  build_gpu_type  = substr(local.build_node_type, 0, 1) == "g" || substr(local.build_node_type, 0, 1) == "p"
  image           = var.image
  release         = coalesce(var.release, local.current)
  tag_map = length(var.tags) == 0 ? {} : {
    for v in split(",", var.tags) :
    "${split("=", v)[0]}" => split("=", v)[1]
  }

  additional_node_groups  = try(jsondecode(var.additional_node_groups_config), jsondecode(base64decode(var.additional_node_groups_config)), [])
  additional_build_groups = try(jsondecode(var.additional_build_groups_config), jsondecode(base64decode(var.additional_build_groups_config)), [])

  additional_karpenter_nodepools = try(jsondecode(var.additional_karpenter_nodepools_config), jsondecode(base64decode(var.additional_karpenter_nodepools_config)), [])

  public_access_cidrs = var.eks_api_server_public_access_cidrs == "" ? ["0.0.0.0/0"] : split(",", var.eks_api_server_public_access_cidrs)
}

module "node_arch" {
  source    = "../../helpers/aws"
  node_type = local.node_type
}

module "build_node_arch" {
  source    = "../../helpers/aws"
  node_type = local.build_node_type
}

module "cluster" {
  source = "../../cluster/aws"

  providers = {
    aws = aws
  }

  additional_node_groups              = local.additional_node_groups
  additional_build_groups             = local.additional_build_groups
  arm_type                            = local.arm_type
  aws_ebs_csi_driver_version          = var.aws_ebs_csi_driver_version
  build_arm_type                      = local.build_arm_type
  availability_zones                  = var.availability_zones
  build_node_enabled                  = var.build_node_enabled
  build_node_min_count                = var.build_node_min_count
  build_node_type                     = var.build_node_type != "" ? var.build_node_type : var.node_type
  cidr                                = var.cidr
  coredns_version                     = var.coredns_version
  disable_public_access               = var.disable_public_access
  enable_private_access               = var.enable_private_access
  ebs_volume_encryption_enabled       = var.ebs_volume_encryption_enabled
  efs_csi_driver_enable               = var.efs_csi_driver_enable
  efs_csi_driver_version              = var.efs_csi_driver_version
  gpu_type                            = local.gpu_type
  build_gpu_type                      = local.build_gpu_type
  gpu_tag_enable                      = var.gpu_tag_enable
  high_availability                   = var.high_availability
  internet_gateway_id                 = var.internet_gateway_id
  imds_http_tokens                    = var.imds_http_tokens
  imds_http_hop_limit                 = var.imds_http_hop_limit
  imds_tags_enable                    = var.imds_tags_enable
  karpenter_auth_mode                 = var.karpenter_auth_mode == "true"
  karpenter_enabled                   = var.karpenter_enabled == "true"
  karpenter_instance_families         = var.karpenter_instance_families
  karpenter_instance_sizes            = var.karpenter_instance_sizes
  karpenter_capacity_types            = var.karpenter_capacity_types
  karpenter_arch                      = var.karpenter_arch != "" ? var.karpenter_arch : (local.arm_type ? "arm64" : "amd64")
  karpenter_cpu_limit                 = var.karpenter_cpu_limit
  karpenter_memory_limit_gb           = var.karpenter_memory_limit_gb
  karpenter_consolidation_enabled     = var.karpenter_consolidation_enabled
  karpenter_consolidate_after         = var.karpenter_consolidate_after
  karpenter_node_expiry               = var.karpenter_node_expiry
  karpenter_disruption_budget_nodes   = var.karpenter_disruption_budget_nodes
  karpenter_node_disk                 = var.karpenter_node_disk
  karpenter_node_volume_type          = var.karpenter_node_volume_type
  karpenter_node_labels               = var.karpenter_node_labels
  karpenter_node_taints               = var.karpenter_node_taints
  karpenter_config                    = var.karpenter_config != "" ? try(base64decode(var.karpenter_config), var.karpenter_config) : "{}"
  karpenter_build_instance_families   = var.karpenter_build_instance_families
  karpenter_build_instance_sizes      = var.karpenter_build_instance_sizes
  karpenter_build_capacity_types      = var.karpenter_build_capacity_types
  karpenter_build_cpu_limit           = var.karpenter_build_cpu_limit
  karpenter_build_memory_limit_gb     = var.karpenter_build_memory_limit_gb
  karpenter_build_consolidate_after   = var.karpenter_build_consolidate_after
  karpenter_build_node_labels         = var.karpenter_build_node_labels
  additional_karpenter_nodepools      = local.additional_karpenter_nodepools
  keda_enable                         = var.keda_enable
  key_pair_name                       = var.key_pair_name
  kube_proxy_version                  = var.kube_proxy_version
  k8s_version                         = var.k8s_version
  max_on_demand_count                 = var.max_on_demand_count
  min_on_demand_count                 = var.min_on_demand_count
  name                                = local.name
  node_capacity_type                  = upper(var.node_capacity_type)
  node_disk                           = var.node_disk
  node_type                           = var.node_type
  node_max_unavailable_percentage     = var.node_max_unavailable_percentage
  terraform_update_timeout            = var.terraform_update_timeout
  nvidia_device_plugin_enable         = var.nvidia_device_plugin_enable
  nvidia_device_time_slicing_replicas = var.nvidia_device_time_slicing_replicas
  private                             = var.private
  private_subnets_ids                 = compact(split(",", var.private_subnets_ids))
  public_subnets_ids                  = compact(split(",", var.public_subnets_ids))
  pod_identity_agent_enable           = var.pod_identity_agent_enable
  pod_identity_agent_version          = var.pod_identity_agent_version
  private_eks_host                    = var.private_eks_host
  private_eks_user                    = var.private_eks_user
  private_eks_pass                    = var.private_eks_pass
  public_access_cidrs                 = local.public_access_cidrs
  kubelet_registry_pull_qps           = var.kubelet_registry_pull_qps
  kubelet_registry_burst              = var.kubelet_registry_burst
  schedule_rack_scale_down            = var.schedule_rack_scale_down
  schedule_rack_scale_up              = var.schedule_rack_scale_up
  tags                                = local.tag_map
  user_data                           = var.user_data
  user_data_url                       = var.user_data_url
  vpc_cni_version                     = var.vpc_cni_version
  vpc_id                              = var.vpc_id
  vpa_enable                          = var.vpa_enable
}

resource "null_resource" "wait_for_cluster" {
  provisioner "local-exec" {
    command = "sleep 1 && echo ${module.cluster.eks_addons[0]}"
  }
}

module "fluentd" {
  source = "../../fluentd/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  depends_on = [
    null_resource.wait_for_cluster
  ]

  access_log_retention_in_days = var.access_log_retention_in_days
  cluster                      = module.cluster.id
  eks_addons                   = module.cluster.eks_addons
  fluentd_disable              = var.fluentd_disable
  fluentd_memory               = var.fluentd_memory
  namespace                    = "kube-system"
  oidc_arn                     = module.cluster.oidc_arn
  oidc_sub                     = module.cluster.oidc_sub
  rack                         = local.name
  syslog                       = var.syslog
}

module "rack" {
  source = "../../rack/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  depends_on = [
    null_resource.wait_for_cluster
  ]

  api_feature_gates                         = var.api_feature_gates
  build_disable_convox_resolver             = var.build_disable_convox_resolver
  karpenter_enabled                         = var.karpenter_enabled == "true"
  build_node_enabled                        = var.build_node_enabled
  buildkit_host_path_cache_enable           = var.buildkit_host_path_cache_enable
  cluster                                   = module.cluster.id
  convox_domain_tls_cert_disable            = var.convox_domain_tls_cert_disable
  convox_rack_domain                        = var.convox_rack_domain
  custom_provided_bucket                    = var.custom_provided_bucket
  deploy_extra_nlb                          = var.deploy_extra_nlb
  docker_hub_username                       = var.docker_hub_username
  docker_hub_password                       = var.docker_hub_password
  disable_convox_resolver                   = var.disable_convox_resolver
  disable_image_manifest_cache              = var.disable_image_manifest_cache
  eks_addons                                = module.cluster.eks_addons
  efs_csi_driver_enable                     = var.efs_csi_driver_enable
  efs_file_system_id                        = module.cluster.efs_file_system_id
  high_availability                         = var.high_availability
  idle_timeout                              = var.idle_timeout
  internal_router                           = var.internal_router
  image                                     = local.image
  keda_enable                               = var.keda_enable
  lbc_helm_id                               = module.cluster.lbc_helm_id
  name                                      = local.name
  rack_name                                 = local.rack_name
  nlb_security_group                        = var.nlb_security_group
  nginx_image                               = var.nginx_image
  nginx_additional_config                   = var.nginx_additional_config
  oidc_arn                                  = module.cluster.oidc_arn
  oidc_sub                                  = module.cluster.oidc_sub
  pdb_default_min_available_percentage      = var.pdb_default_min_available_percentage
  proxy_protocol                            = var.proxy_protocol
  release                                   = local.release
  releases_to_retain_after_active           = var.releases_to_retain_after_active
  releases_to_retain_task_run_interval_hour = var.releases_to_retain_task_run_interval_hour
  ssl_ciphers                               = var.ssl_ciphers
  ssl_protocols                             = var.ssl_protocols
  subnets                                   = module.cluster.subnets
  tags                                      = local.tag_map
  telemetry                                 = var.telemetry
  telemetry_map                             = local.telemetry_map
  telemetry_default_map                     = local.telemetry_default_map
  whitelist                                 = split(",", var.whitelist)
  ecr_scan_on_push_enable                   = var.ecr_scan_on_push_enable
  vpc_id                                    = module.cluster.vpc
  vpa_enable                                = var.vpa_enable
}
