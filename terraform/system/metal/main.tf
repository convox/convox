data "http" "releases" {
  count = var.release == "" ? 1 : 0

  url = "https://api.github.com/repos/${var.image}/releases/latest"
  request_headers = {
    User-Agent = "convox"
  }
}

locals {
  name            = lower(var.name)
  rack_name       = lower(var.rack_name)
  desired_release = var.release != "" ? var.release : jsondecode(data.http.releases[0].response_body).tag_name
  release         = local.desired_release
}

provider "kubernetes" {}

module "rack" {
  source = "../../rack/metal"

  providers = {
    kubernetes = kubernetes
  }

  docker_hub_username      = var.docker_hub_username
  docker_hub_password      = var.docker_hub_password
  domain                   = var.domain
  elasticsearch_disk       = var.elasticsearch_disk
  fluentd_memory           = var.fluentd_memory
  image                    = var.image
  name                     = local.name
  release                  = local.release
  registry_disk            = var.registry_disk
  router_log_redact_params = var.router_log_redact_params
  syslog                   = var.syslog
  syslog_tls_verify        = var.syslog_tls_verify
  webhook_signing_key      = var.webhook_signing_key
  whitelist                = split(",", var.whitelist)
}
