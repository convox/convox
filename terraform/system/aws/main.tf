provider "aws" {
  region = var.region
}

provider "kubernetes" {
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    args        = ["eks", "get-token", "--cluster-name", var.name]
    command     = "aws"
  }
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
  // var.node_type can be assigned a comma separated list of instance types
  node_type = split(",", var.node_type)[0]
  arm_type  = substr(local.node_type, 0, 2) == "a1" || substr(local.node_type, 0, 3) == "c6g" || substr(local.node_type, 0, 3) == "c7g" || substr(local.node_type, 0, 3) == "m6g" || substr(local.node_type, 0, 3) == "r6g" || substr(local.node_type, 0, 3) == "t4g"
  current   = jsondecode(data.http.releases.response_body).tag_name
  gpu_type  = substr(local.node_type, 0, 1) == "g" || substr(local.node_type, 0, 1) == "p"
  image     = var.image
  release   = local.arm_type ? format("%s-%s", coalesce(var.release, local.current), "arm64") : coalesce(var.release, local.current)
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

  arm_type                 = local.arm_type
  availability_zones       = var.availability_zones
  build_node_enabled       = var.build_node_enabled
  build_node_min_count     = var.build_node_min_count
  build_node_type          = var.build_node_type != "" ? var.build_node_type : var.node_type
  cidr                     = var.cidr
  coredns_version          = var.coredns_version
  gpu_type                 = local.gpu_type
  gpu_tag_enable           = var.gpu_tag_enable
  high_availability        = var.high_availability
  internet_gateway_id      = var.internet_gateway_id
  imds_http_tokens         = var.imds_http_tokens
  key_pair_name            = var.key_pair_name
  kube_proxy_version       = var.kube_proxy_version
  k8s_version              = var.k8s_version
  max_on_demand_count      = var.max_on_demand_count
  min_on_demand_count      = var.min_on_demand_count
  name                     = var.name
  node_capacity_type       = upper(var.node_capacity_type)
  node_disk                = var.node_disk
  node_type                = var.node_type
  private                  = var.private
  schedule_rack_scale_down = var.schedule_rack_scale_down
  schedule_rack_scale_up   = var.schedule_rack_scale_up
  tags                     = local.tag_map
  vpc_cni_version          = var.vpc_cni_version
  vpc_id                   = var.vpc_id
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

  arm_type   = local.arm_type
  cluster    = module.cluster.id
  eks_addons = module.cluster.eks_addons
  namespace  = "kube-system"
  oidc_arn   = module.cluster.oidc_arn
  oidc_sub   = module.cluster.oidc_sub
  rack       = var.name
  syslog     = var.syslog
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

  build_node_enabled  = var.build_node_enabled
  cluster             = module.cluster.id
  docker_hub_username = var.docker_hub_username
  docker_hub_password = var.docker_hub_password
  eks_addons          = module.cluster.eks_addons
  high_availability   = var.high_availability
  idle_timeout        = var.idle_timeout
  internal_router     = var.internal_router
  image               = local.image
  name                = var.name
  rack_name           = var.rack_name
  oidc_arn            = module.cluster.oidc_arn
  oidc_sub            = module.cluster.oidc_sub
  proxy_protocol      = var.proxy_protocol
  release             = local.release
  settings            = var.settings
  ssl_ciphers         = var.ssl_ciphers
  ssl_protocols       = var.ssl_protocols
  subnets             = module.cluster.subnets
  tags                = local.tag_map
  telemetry           = var.telemetry
  whitelist           = split(",", var.whitelist)
  ebs_csi_driver_name = module.cluster.ebs_csi_driver_name
}