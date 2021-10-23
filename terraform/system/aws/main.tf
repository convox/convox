provider "aws" {
  region = var.region
}

provider "kubernetes" {
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint

  load_config_file = false
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
  current = jsondecode(data.http.releases.body).tag_name
  release = coalesce(var.release, local.current)
}

module "cluster" {
  source = "../../cluster/aws"

  providers = {
    aws = aws
  }

  availability_zones = var.availability_zones
  cidr               = var.cidr
  k8s_version        = var.k8s_version
  name               = var.name
  node_disk          = var.node_disk
  node_type          = var.node_type
  private            = var.private
}

module "fluentd" {
  source = "../../fluentd/aws"

  providers = {
    aws        = aws
    kubernetes = kubernetes
  }

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
  idle_timeout        = var.idle_timeout
  image               = var.image
  name                = var.name
  oidc_arn            = module.cluster.oidc_arn
  oidc_sub            = module.cluster.oidc_sub
  release             = local.release
  subnets             = module.cluster.subnets
  whitelist           = split(",", var.whitelist)
}
