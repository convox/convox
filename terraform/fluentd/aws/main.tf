data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

locals {
  buffer_size        = max(8, floor((local.memory_mi * 0.95 - 190) / 2))
  memory_mi          = try(tonumber(regex("^([0-9]+)Mi$", var.fluentd_memory)[0]), tonumber(regex("^([0-9]+)Gi$", var.fluentd_memory)[0]) * 1024, 200)
  redact_names       = [for n in split(",", trimspace(var.router_log_redact_params)) : trimspace(n) if length(regexall("^[A-Za-z0-9_.-]+$", trimspace(n))) > 0 && lower(trimspace(n)) != "all"]
  redact_mode        = trimspace(var.router_log_redact_params) == "all" ? "all" : length(local.redact_names) > 0 ? "names" : ""
  redact_alternation = join("|", [for n in local.redact_names : replace(replace(n, ".", "\\."), "_", "(?:_|%5F)")])

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

  cluster   = var.cluster
  image     = "convox/fluentd:1.19.3-all@sha256:b00e07590ded273d07e2bcd33e6ecca771f74637bcdb45c80446a35b222a3c48"
  namespace = var.namespace
  rack      = var.rack

  target = templatefile("${path.module}/target.conf.tpl", {
    access_log_retention   = var.access_log_retention_in_days,
    app_cloudwatch_disable = var.app_cloudwatch_disable,
    buffer_size            = local.buffer_size,
    rack                   = var.rack,
    redact_alternation     = local.redact_alternation,
    redact_mode            = local.redact_mode,
    region                 = data.aws_region.current.name,
    syslog                 = compact(split(",", var.syslog)),
    syslog_access_logs     = var.syslog_access_logs,
    syslog_tls_verify      = var.syslog_tls_verify,
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
