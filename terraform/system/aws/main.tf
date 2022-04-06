provider "aws" {
  region = var.region
}

provider "kubernetes" {
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint

  exec {
    api_version = "client.authentication.k8s.io/v1alpha1"
    args        = ["eks", "get-token", "--cluster-name", var.name]
    command     = "aws"
  }
}

data "aws_eks_cluster_auth" "cluster" {
  name = module.cluster.id
}

data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  // var.node_type can be assigned a comma separated list of instance types
  node_type = split(",", var.node_type)[0]
  arm_type  = local.node_type == "a1" || substr(local.node_type, 0, 3) == "c6g" || substr(local.node_type, 0, 3) == "m6g" || substr(local.node_type, 0, 3) == "r6g" || substr(local.node_type, 0, 3) == "t4g"
  current   = jsondecode(data.http.releases.body).tag_name
  gpu_type  = substr(local.node_type, 0, 1) == "g" || substr(local.node_type, 0, 1) == "p"
  image     = var.image
  release   = local.arm_type ? format("%s-%s", coalesce(var.release, local.current), "arm64") : coalesce(var.release, local.current)
}

module "cluster" {
  source = "../../cluster/aws"

  providers = {
    aws = aws
  }

  arm_type            = local.arm_type
  availability_zones  = var.availability_zones
  cidr                = var.cidr
  gpu_type            = local.gpu_type
  high_availability   = var.high_availability
  internet_gateway_id = var.internet_gateway_id
  k8s_version         = var.k8s_version
  name                = var.name
  node_capacity_type  = upper(var.node_capacity_type)
  node_disk           = var.node_disk
  node_type           = var.node_type
  private             = var.private
  vpc_id              = var.vpc_id
}

module "fluentd" {
  source = "../../fluentd/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  arm_type  = local.arm_type
  cluster   = module.cluster.id
  namespace = "kube-system"
  oidc_arn  = module.cluster.oidc_arn
  oidc_sub  = module.cluster.oidc_sub
  rack      = var.name
  syslog    = var.syslog
}

module "rack" {
  source = "../../rack/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

  cluster             = module.cluster.id
  docker_hub_username = var.docker_hub_username
  docker_hub_password = var.docker_hub_password
  high_availability   = var.high_availability
  idle_timeout        = var.idle_timeout
  image               = local.image
  name                = var.name
  oidc_arn            = module.cluster.oidc_arn
  oidc_sub            = module.cluster.oidc_sub
  release             = local.release
  subnets             = module.cluster.subnets
  whitelist           = split(",", var.whitelist)
}
