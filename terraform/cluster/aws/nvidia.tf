resource "helm_release" "nvidia_device_plugin" {
  depends_on = [
    null_resource.wait_k8s_api,
  ]

  count = var.nvidia_device_plugin_enable ? 1 : 0

  name       = "nvidia-device-plugin"
  repository = "https://nvidia.github.io/k8s-device-plugin"
  chart      = "nvidia-device-plugin"
  version    = "0.17.1"
  namespace  = "kube-system"
}
