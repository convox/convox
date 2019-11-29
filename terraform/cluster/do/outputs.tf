output "ca" {
  depends_on = [digitalocean_kubernetes_cluster.rack]
  value      = base64decode(digitalocean_kubernetes_cluster.rack.kube_config[0].cluster_ca_certificate)
}

output "client_certificate" {
  depends_on = [digitalocean_kubernetes_cluster.rack]
  value      = digitalocean_kubernetes_cluster.rack.kube_config[0].client_certificate
}

output "client_key" {
  depends_on = [digitalocean_kubernetes_cluster.rack]
  value      = digitalocean_kubernetes_cluster.rack.kube_config[0].client_key
}

output "endpoint" {
  depends_on = [digitalocean_kubernetes_cluster.rack]
  value      = digitalocean_kubernetes_cluster.rack.endpoint
}

output "name" {
  depends_on = [digitalocean_kubernetes_cluster.rack]
  value      = digitalocean_kubernetes_cluster.rack.name
}
