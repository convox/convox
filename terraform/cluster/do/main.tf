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
    min_nodes  = var.high_availability ? 2 : 1
    max_nodes  = var.high_availability ? 10 : 3
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

resource "local_file" "kubeconfig" {
  depends_on = [digitalocean_kubernetes_cluster.rack, null_resource.delay_token]

  filename = pathexpand("~/.kube/config.do.${var.name}")
  content = templatefile("${path.module}/kubeconfig.tpl", {
    ca       = digitalocean_kubernetes_cluster.rack.kube_config[0].cluster_ca_certificate
    endpoint = digitalocean_kubernetes_cluster.rack.endpoint
    token    = digitalocean_kubernetes_cluster.rack.kube_config[0].token
  })
}
