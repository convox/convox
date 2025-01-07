data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  tags = {
    System = "convox"
    Rack   = var.name
  }
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  buildkit_enabled               = var.buildkit_enabled
  build_node_enabled             = var.build_node_enabled
  convox_domain_tls_cert_disable = var.convox_domain_tls_cert_disable
  docker_hub_authentication      = var.docker_hub_authentication
  docker_hub_username            = var.docker_hub_username
  docker_hub_password            = var.docker_hub_password
  domain                         = var.domain
  domain_internal                = var.domain_internal
  disable_image_manifest_cache   = var.disable_image_manifest_cache
  image                          = var.image
  metrics_scraper_host           = var.metrics_scraper_host
  namespace                      = var.namespace
  rack                           = var.name
  rack_name                      = var.rack_name
  release                        = var.release
  replicas                       = var.high_availability ? 2 : 1
  resolver                       = var.resolver

  annotations = {
    "cert-manager.io/cluster-issuer" = "letsencrypt"
    "cert-manager.io/duration"       = var.cert_duration
    "eks.amazonaws.com/role-arn"     = aws_iam_role.api.arn
    "iam.amazonaws.com/role"         = aws_iam_role.api.arn
  }

  env = {
    AWS_REGION                           = data.aws_region.current.name
    BUCKET                               = aws_s3_bucket.storage.id
    CERT_MANAGER                         = "true"
    CERT_MANAGER_ROLE_ARN                = aws_iam_role.cert-manager.arn
    EFS_FILE_SYSTEM_ID                   = var.efs_file_system_id
    BUILD_DISABLE_CONVOX_RESOLVER        = var.build_disable_convox_resolver
    PDB_DEFAULT_MIN_AVAILABLE_PERCENTAGE = var.pdb_default_min_available_percentage
    PROVIDER                             = "aws"
    RESOLVER                             = var.resolver
    ROUTER                               = var.router
    SOCKET                               = "/var/run/docker.sock"
    ECR_SCAN_ON_PUSH_ENABLE              = var.ecr_scan_on_push_enable
    SUBNET_IDS                           = join(",", var.subnets)
    VPC_ID                               = var.vpc_id
  }
}


// efs related resources

resource "kubernetes_persistent_volume_claim_v1" "efs-pvc-system-775" {
  count = var.efs_file_system_id != "" ? 1 : 0

  metadata {
    name = "efs-pvc-system-775"
    namespace = var.namespace
  }

  spec {
    access_modes = [ "ReadWriteMany" ]
    resources {
      requests = {
        storage = "2Gi"
      }
    }
    volume_name = "efs-pv-775"
    storage_class_name = "efs-sc-base"
  }
}
