output "ca" {
  value = base64decode(digitalocean_kubernetes_cluster.rack.kube_config[0].cluster_ca_certificate)
}

output "endpoint" {
  value = digitalocean_kubernetes_cluster.rack.endpoint
}

output "id" {
  value = digitalocean_kubernetes_cluster.rack.id
}

output "name" {
  value = digitalocean_kubernetes_cluster.rack.name
}

output "token" {
  depends_on = [null_resource.delay_token]
  value      = digitalocean_kubernetes_cluster.rack.kube_config[0].token
}
