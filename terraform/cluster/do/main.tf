terraform {
  required_version = ">= 0.12.0"
}

provider "digitalocean" {
  version = "~> 1.11"
}

provider "http" {
  version = "~> 1.1"
}

provider "local" {
  version = "~> 1.3"
}

provider "random" {
  version = "~> 2.2"
}

data "http" "kubernetes_versions" {
  url = "https://api.digitalocean.com/v2/kubernetes/options"

  request_headers = {
    Authorization = "Bearer ${var.token}"
  }
}

locals {
  kubernetes_desired = "1.14"
  kubernetes_slug    = [for v in jsondecode(data.http.kubernetes_versions.body).options.versions : v.slug if length(regexall("^${local.kubernetes_desired}\\.", v.kubernetes_version)) > 0].0
}

resource "digitalocean_kubernetes_cluster" "rack" {
  name    = var.name
  region  = var.region
  version = local.kubernetes_slug

  node_pool {
    name       = "${var.name}-node"
    size       = var.node_type
    auto_scale = true
    min_nodes  = 1
    max_nodes  = 10
  }
}

resource "local_file" "kubeconfig" {
  depends_on = [digitalocean_kubernetes_cluster.rack]

  filename = pathexpand("~/.kube/config.do.${var.name}")
  content = templatefile("${path.module}/kubeconfig.tpl", {
    ca                 = digitalocean_kubernetes_cluster.rack.kube_config[0].cluster_ca_certificate
    client_certificate = base64encode(digitalocean_kubernetes_cluster.rack.kube_config[0].client_certificate)
    client_key         = base64encode(digitalocean_kubernetes_cluster.rack.kube_config[0].client_key)
    endpoint           = digitalocean_kubernetes_cluster.rack.endpoint
  })
}
