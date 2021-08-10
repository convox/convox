output "namespace" {
  depends_on = [kubernetes_namespace.system]
  value      = kubernetes_namespace.system.metadata[0].name
}

output "docker_hub_authentication" {
  depends_on = [kubernetes_secret.docker_hub_authentication]
  value      = length(kubernetes_secret.docker_hub_authentication) > 0 ? kubernetes_secret.docker_hub_authentication[0].metadata[0].name : null
}
