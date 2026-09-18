provider "google" {
  project = module.project.id
  region  = var.region
}

provider "kubernetes" {
  client_certificate     = module.cluster.client_certificate
  client_key             = module.cluster.client_key
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint

}

module "project" {
  source = "./project"
}

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

  # a malformed non-empty value must fail the plan, not silently become [] and destroy the pools
  additional_node_groups = var.additional_node_groups_config == "" ? [] : try(jsondecode(var.additional_node_groups_config), jsondecode(base64decode(var.additional_node_groups_config)))
}

module "cluster" {
  source = "../../cluster/gcp"

  providers = {
    google = google
  }

  additional_node_groups          = local.additional_node_groups
  k8s_version                     = var.k8s_version
  name                            = local.name
  node_disk                       = var.node_disk
  node_type                       = var.node_type
  terraform_update_timeout        = var.terraform_update_timeout
  preemptible                     = var.preemptible
  project_id                      = module.project.id
  gpu_observability_enable        = var.gpu_observability_enable
  gpu_observability_chart_version = var.gpu_observability_chart_version
  dcgm_scrape_interval            = var.dcgm_scrape_interval
}

module "rack" {
  source = "../../rack/gcp"

  providers = {
    kubernetes = kubernetes
    google     = google
  }

  buildkit_enabled        = var.buildkit_enabled
  cluster                 = module.cluster.id
  docker_hub_username     = var.docker_hub_username
  docker_hub_password     = var.docker_hub_password
  fluentd_memory          = var.fluentd_memory
  image                   = var.image
  name                    = local.name
  rack_name               = local.rack_name
  network                 = module.cluster.network
  nodes_account           = module.cluster.nodes_account
  nginx_additional_config = var.nginx_additional_config
  project_id              = module.project.id
  region                  = var.region
  release                 = local.release
  syslog                  = var.syslog
  telemetry               = var.telemetry
  telemetry_map           = local.telemetry_map
  telemetry_default_map   = local.telemetry_default_map
  webhook_signing_key     = var.webhook_signing_key
  whitelist               = split(",", var.whitelist)
}
