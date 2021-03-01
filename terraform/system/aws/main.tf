provider "aws" {
  region = var.region
}

provider "kubernetes" {
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint
  token                  = data.aws_eks_cluster_auth.cluster.token

  load_config_file = false
}

data "aws_eks_cluster_auth" "cluster" {
  name = module.cluster.id
}

data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases/latest"
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

  cidr       = var.cidr
  name       = var.name
  node_disk  = var.node_disk
  node_type  = var.node_type
  private    = var.private
  spot_types = var.spot_types
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

  cluster   = module.cluster.id
  name      = var.name
  oidc_arn  = module.cluster.oidc_arn
  oidc_sub  = module.cluster.oidc_sub
  release   = local.release
  subnets   = module.cluster.subnets
  whitelist = split(",", var.whitelist)
}
