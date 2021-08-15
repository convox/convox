output "namespace" {
  depends_on = [kubernetes_namespace.system]
  value      = kubernetes_namespace.system.metadata[0].name
}
