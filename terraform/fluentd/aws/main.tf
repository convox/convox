data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  tags = {
    System  = "convox"
    Cluster = var.cluster
  }
}

module "k8s" {
  source = "../k8s"

  providers = {
    kubernetes = kubernetes
  }

  fluentd_disable = var.fluentd_disable
  fluentd_memory  = var.fluentd_memory
  node_name_env   = true

  cluster = var.cluster
  # 1.19-all runs fluentd 1.19 with kubernetes_metadata_filter 3.8, which re-reads the
  # service account token (1.13-all runs fluentd 1.7.4 with filter 2.3.0, which reads it
  # once; EKS 1.34+ expires it after 24 hours and every app log line is then dropped).
  image     = "convox/fluentd:1.19-all"
  namespace = var.namespace
  rack      = var.rack

  target = templatefile("${path.module}/target.conf.tpl", {
    access_log_retention   = var.access_log_retention_in_days,
    app_cloudwatch_disable = var.app_cloudwatch_disable,
    rack                   = var.rack,
    region                 = data.aws_region.current.name,
    syslog                 = compact(split(",", var.syslog))
  })

  annotations = {
    "eks.amazonaws.com/role-arn" = aws_iam_role.fluentd.arn,
    "iam.amazonaws.com/role"     = aws_iam_role.fluentd.arn,
    "convox.com/dummy"           = var.access_log_retention_in_days,
  }

  env = {
    AWS_REGION = data.aws_region.current.name
  }
}
