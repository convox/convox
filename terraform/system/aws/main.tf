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
  arm_type        = substr(local.node_type, 0, 2) == "a1" || substr(local.node_type, 0, 3) == "c6g" || substr(local.node_type, 0, 3) == "c7g" || substr(local.node_type, 0, 3) == "m6g" || substr(local.node_type, 0, 3) == "r6g" || substr(local.node_type, 0, 3) == "t4g"
  build_arm_type  = substr(local.build_node_type, 0, 2) == "a1" || substr(local.build_node_type, 0, 3) == "c6g" || substr(local.build_node_type, 0, 3) == "c7g" || substr(local.build_node_type, 0, 3) == "m6g" || substr(local.build_node_type, 0, 3) == "r6g" || substr(local.build_node_type, 0, 3) == "t4g"
  current         = jsondecode(data.http.releases.response_body).tag_name
  gpu_type        = substr(local.node_type, 0, 1) == "g" || substr(local.node_type, 0, 1) == "p"
  build_gpu_type  = substr(local.build_node_type, 0, 1) == "g" || substr(local.build_node_type, 0, 1) == "p"
  image           = var.image
  release         = local.arm_type ? format("%s-%s", coalesce(var.release, local.current), "arm64") : coalesce(var.release, local.current)
  tag_map = length(var.tags) == 0 ? {} : {
    for v in split(",", var.tags) :
    "${split("=", v)[0]}" => split("=", v)[1]
  }
}

module "cluster" {
  source = "../../cluster/aws"

  providers = {
    aws = aws
  }

  arm_type                        = local.arm_type
  aws_ebs_csi_driver_version      = var.aws_ebs_csi_driver_version
  build_arm_type                  = local.build_arm_type
  availability_zones              = var.availability_zones
  build_node_enabled              = var.build_node_enabled
  build_node_min_count            = var.build_node_min_count
  build_node_type                 = var.build_node_type != "" ? var.build_node_type : var.node_type
  cidr                            = var.cidr
  coredns_version                 = var.coredns_version
  disable_public_access           = var.disable_public_access
  efs_csi_driver_enable           = var.efs_csi_driver_enable
  efs_csi_driver_version          = var.efs_csi_driver_version
  gpu_type                        = local.gpu_type
  build_gpu_type                  = local.build_gpu_type
  gpu_tag_enable                  = var.gpu_tag_enable
  high_availability               = var.high_availability
  internet_gateway_id             = var.internet_gateway_id
  imds_http_tokens                = var.imds_http_tokens
  imds_http_hop_limit             = var.imds_http_hop_limit
  imds_tags_enable                = var.imds_tags_enable
  key_pair_name                   = var.key_pair_name
  kube_proxy_version              = var.kube_proxy_version
  k8s_version                     = var.k8s_version
  max_on_demand_count             = var.max_on_demand_count
  min_on_demand_count             = var.min_on_demand_count
  name                            = local.name
  node_capacity_type              = upper(var.node_capacity_type)
  node_disk                       = var.node_disk
  node_type                       = var.node_type
  node_max_unavailable_percentage = var.node_max_unavailable_percentage
  private                         = var.private
  private_subnets_ids             = compact(split(",", var.private_subnets_ids))
  public_subnets_ids              = compact(split(",", var.public_subnets_ids))
  pod_identity_agent_enable       = var.pod_identity_agent_enable
  pod_identity_agent_version      = var.pod_identity_agent_version
  private_eks_host                = var.private_eks_host
  private_eks_user                = var.private_eks_user
  private_eks_pass                = var.private_eks_pass
  kubelet_registry_pull_qps       = var.kubelet_registry_pull_qps
  kubelet_registry_burst          = var.kubelet_registry_burst
  schedule_rack_scale_down        = var.schedule_rack_scale_down
  schedule_rack_scale_up          = var.schedule_rack_scale_up
  tags                            = local.tag_map
  user_data                       = var.user_data
  user_data_url                   = var.user_data_url
  vpc_cni_version                 = var.vpc_cni_version
  vpc_id                          = var.vpc_id
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
  arm_type                     = local.arm_type
  cluster                      = module.cluster.id
  eks_addons                   = module.cluster.eks_addons
  fluentd_disable              = var.fluentd_disable
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

  build_disable_convox_resolver        = var.build_disable_convox_resolver
  build_node_enabled                   = var.build_node_enabled
  cluster                              = module.cluster.id
  convox_domain_tls_cert_disable       = var.convox_domain_tls_cert_disable
  convox_rack_domain                   = var.convox_rack_domain
  deploy_extra_nlb                     = var.deploy_extra_nlb
  docker_hub_username                  = var.docker_hub_username
  docker_hub_password                  = var.docker_hub_password
  disable_image_manifest_cache         = var.disable_image_manifest_cache
  eks_addons                           = module.cluster.eks_addons
  efs_csi_driver_enable                = var.efs_csi_driver_enable
  efs_file_system_id                   = module.cluster.efs_file_system_id
  high_availability                    = var.high_availability
  idle_timeout                         = var.idle_timeout
  internal_router                      = var.internal_router
  image                                = local.image
  lbc_helm_id                          = module.cluster.lbc_helm_id
  name                                 = local.name
  rack_name                            = local.rack_name
  nlb_security_group                   = var.nlb_security_group
  nginx_image                          = var.nginx_image
  oidc_arn                             = module.cluster.oidc_arn
  oidc_sub                             = module.cluster.oidc_sub
  pdb_default_min_available_percentage = var.pdb_default_min_available_percentage
  proxy_protocol                       = var.proxy_protocol
  release                              = local.release
  ssl_ciphers                          = var.ssl_ciphers
  ssl_protocols                        = var.ssl_protocols
  subnets                              = module.cluster.subnets
  tags                                 = local.tag_map
  telemetry                            = var.telemetry
  telemetry_map                        = local.telemetry_map
  telemetry_default_map                = local.telemetry_default_map
  whitelist                            = split(",", var.whitelist)
  ecr_scan_on_push_enable              = var.ecr_scan_on_push_enable
  vpc_id                               = module.cluster.vpc
}
