terraform {
  required_version = ">= 0.12.0"
}

provider "digitalocean" {
  version = "~> 1.9"
}

provider "local" {
  version = "~> 1.3"
}

provider "random" {
  version = "~> 2.2"
}

resource "digitalocean_kubernetes_cluster" "rack" {
  name    = var.name
  region  = var.region
  version = "1.14.8-do.0"

  node_pool {
    name       = "rack"
    size       = var.node_type
    auto_scale = true
    min_nodes  = 1
    max_nodes  = 10
  }
}

resource "local_file" "kubeconfig" {
  depends_on = [
    digitalocean_kubernetes_cluster.rack,
  ]

  filename = pathexpand("~/.kube/config.do.${var.name}")
  content = templatefile("${path.module}/kubeconfig.tpl", {
    ca       = digitalocean_kubernetes_cluster.rack.kube_config[0].cluster_ca_certificate
    endpoint = digitalocean_kubernetes_cluster.rack.endpoint
    token    = digitalocean_kubernetes_cluster.rack.kube_config[0].token
  })

  # lifecycle {
  #   ignore_changes = [content]
  # }
}
