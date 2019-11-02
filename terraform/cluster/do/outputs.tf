output "kubeconfig" {
  depends_on = [
    local_file.kubeconfig,
    kubernetes_cluster_role_binding.client,
    digitalocean_kubernetes_cluster.rack,
  ]
  value = local_file.kubeconfig.filename
}
