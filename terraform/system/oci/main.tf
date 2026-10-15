provider "oci" {
  fingerprint  = var.fingerprint
  private_key  = var.private_key
  region       = var.region
  tenancy_ocid = var.tenancy_ocid
  user_ocid    = var.user_ocid
}

# oke only issues exec kubeconfigs, so the oci cli must be installed and able to
# read the same credentials as the provider above
provider "kubernetes" {
  cluster_ca_certificate = module.cluster.ca
  host                   = module.cluster.endpoint

  exec {
    api_version = "client.authentication.k8s.io/v1beta1"
    command     = "oci"
    args        = ["ce", "cluster", "generate-token", "--cluster-id", module.cluster.id, "--region", var.region]
  }
}

data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
}

locals {
  name             = lower(var.name)
  rack_name        = lower(var.rack_name)
  current          = jsondecode(data.http.releases.response_body).tag_name
  release          = coalesce(var.release, local.current)
  compartment_ocid = var.compartment_ocid != "" ? var.compartment_ocid : var.tenancy_ocid
}

module "cluster" {
  source = "../../cluster/oci"

  providers = {
    oci = oci
  }

  compartment_ocid         = local.compartment_ocid
  gpu_node_count           = var.gpu_node_count
  gpu_node_type            = var.gpu_node_type
  k8s_version              = var.k8s_version
  name                     = local.name
  node_count               = var.node_count
  node_disk                = var.node_disk
  node_memory              = var.node_memory
  node_ocpus               = var.node_ocpus
  node_type                = var.node_type
  region                   = var.region
  terraform_update_timeout = var.terraform_update_timeout
}

module "rack" {
  source = "../../rack/oci"

  providers = {
    oci        = oci
    kubernetes = kubernetes
  }

  buildkit_enabled      = var.buildkit_enabled
  cluster               = module.cluster.id
  compartment_ocid      = local.compartment_ocid
  docker_hub_username   = var.docker_hub_username
  docker_hub_password   = var.docker_hub_password
  fluentd_memory        = var.fluentd_memory
  high_availability     = var.high_availability
  image                 = var.image
  name                  = local.name
  rack_name             = local.rack_name
  region                = var.region
  release               = local.release
  syslog                = var.syslog
  telemetry             = var.telemetry
  telemetry_map         = local.telemetry_map
  telemetry_default_map = local.telemetry_default_map
  tenancy_ocid          = var.tenancy_ocid
  webhook_signing_key   = var.webhook_signing_key
  whitelist             = split(",", var.whitelist)
}
