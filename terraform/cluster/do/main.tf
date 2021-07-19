data "digitalocean_kubernetes_versions" "available" {
  version_prefix = "1.20."
}

resource "digitalocean_kubernetes_cluster" "rack" {
  name    = var.name
  region  = var.region
  version = data.digitalocean_kubernetes_versions.available.latest_version

  node_pool {
    name       = "${var.name}-node"
    size       = var.node_type
    auto_scale = true
    min_nodes  = 2
    max_nodes  = 10
  }
}

# new tokens sometimes take a few seconds to start working
resource "null_resource" "delay_token" {
  provisioner "local-exec" {
    command = "sleep 30"
  }
  triggers = {
    token = digitalocean_kubernetes_cluster.rack.kube_config[0].token
  }
}
