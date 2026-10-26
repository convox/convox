output "namespace" {
  depends_on = [kubernetes_namespace.system]
  value      = kubernetes_namespace.system.metadata[0].name
}

output "registry_image" {
  value = "registry:3.1.2@sha256:01cd312e67026f06f5d89c3e133a4a9adc7cb9eeb9b1194e0b2954dfd770efd2"
}

output "docker_hub_authentication" {
  depends_on = [kubernetes_secret.docker_hub_authentication]
  value      = length(kubernetes_secret.docker_hub_authentication) > 0 ? kubernetes_secret.docker_hub_authentication[0].metadata[0].name : "NULL"
}
