data "http" "releases" {
  url = "https://api.github.com/repos/convox/convox/releases/latest"
  request_headers = {
    User-Agent = "convox"
  }
}

locals {
  current         = jsondecode(data.http.releases.response_body).tag_name
  release         = coalesce(var.release, local.current)
  kube_config_yaml = yamldecode(module.cluster.kubeconfigraw)
  s3_region_endpoint = "https://sos-${var.zone}.exo.io"
}

provider "aws" {

  endpoints {
    s3 = local.s3_region_endpoint
  }

  region     = var.zone

  access_key = var.exoscale_api_key
  secret_key = var.exoscale_api_secret

  # Disable AWS-specific features
  skip_credentials_validation = true
  skip_region_validation      = true
  skip_requesting_account_id  = true
  skip_metadata_api_check = true
  # add this when we update aws terraform provider version
  # skip_s3_checksum            = true
}

provider "exoscale" {
  key    = var.exoscale_api_key
  secret = var.exoscale_api_secret
}

provider "kubernetes" {
  host = local.kube_config_yaml.clusters[0].cluster.server

  client_certificate     = base64decode(local.kube_config_yaml.users[0].user.client-certificate-data)
  client_key             = base64decode(local.kube_config_yaml.users[0].user.client-key-data)
  cluster_ca_certificate = base64decode(local.kube_config_yaml.clusters[0].cluster.certificate-authority-data)
}

module "cluster" {
  source = "../../cluster/exoscale"

  providers = {
    exoscale = exoscale
  }

  high_availability = var.high_availability
  k8s_version       = var.k8s_version
  name              = var.name
  instance_type =  var.instance_type
  instance_disk_size = var.instance_disk_size
  zone            = var.zone
}

module "rack" {
  source = "../../rack/exoscale"

  providers = {
    aws = aws
    kubernetes = kubernetes
    exoscale   = exoscale
  }

  build_node_enabled           = var.build_node_enabled
  cluster                      = module.cluster.depend_id
  docker_hub_username          = var.docker_hub_username
  docker_hub_password          = var.docker_hub_password
  disable_image_manifest_cache = var.disable_image_manifest_cache
  high_availability            = var.high_availability
  image                        = var.image
  name                         = var.name
  rack_name                    = var.rack_name
  release                      = local.release
  ssl_ciphers                  = var.ssl_ciphers
  ssl_protocols                = var.ssl_protocols
  telemetry                    = var.telemetry
  telemetry_map                = local.telemetry_map
  telemetry_default_map        = local.telemetry_default_map
  whitelist                    = split(",", var.whitelist)
  registry_disk = var.registry_disk
  zone = var.zone
}
