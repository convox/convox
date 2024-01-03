data "http" "releases" {
  url = "https://api.github.com/repos/${var.image}/releases/latest"
  request_headers = {
    User-Agent = "convox"
  }
}

locals {
  current         = jsondecode(data.http.releases.response_body).tag_name
  release         = coalesce(var.release, local.current)
  kube_config_yaml = yamldecode(module.cluster.kubeconfigraw)
}

provider "kubernetes" {
  host = local.kube_config_yaml.clusters[0].cluster.server

  client_certificate     = base64decode(local.kube_config_yaml.users[0].user.client-certificate-data)
  client_key             = base64decode(local.kube_config_yaml.users[0].user.client-key-data)
  cluster_ca_certificate = base64decode(local.kube_config_yaml.clusters[0].cluster.certificate-authority-data)
}

module "cluster" {
  source = "../../cluster/exoscale"

  high_availability = var.high_availability
  k8s_version       = var.k8s_version
  name              = var.name
  instance_type =  var.instance_type
  instance_disk_size = var.instance_disk_size
  zone            = var.zone
}
