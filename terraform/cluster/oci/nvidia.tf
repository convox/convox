resource "helm_release" "nvidia_device_plugin" {
  depends_on = [
    oci_containerengine_node_pool.gpu,
  ]

  count = var.gpu_node_type != "" ? 1 : 0

  name       = "nvidia-device-plugin"
  repository = "https://nvidia.github.io/k8s-device-plugin"
  chart      = "nvidia-device-plugin"
  version    = "0.17.1"
  namespace  = "kube-system"

  values = [
    yamlencode({
      affinity = {
        nodeAffinity = {
          requiredDuringSchedulingIgnoredDuringExecution = {
            nodeSelectorTerms = [
              {
                matchExpressions = [
                  {
                    key      = "convox.io/gpu-vendor"
                    operator = "In"
                    values   = ["nvidia"]
                  }
                ]
              },
            ]
          }
        }
      }
      tolerations = [
        {
          key      = "CriticalAddonsOnly"
          operator = "Exists"
        },
        {
          key      = "nvidia.com/gpu"
          operator = "Exists"
          effect   = "NoSchedule"
        },
        {
          key      = "dedicated-node"
          operator = "Exists"
          effect   = "NoSchedule"
        },
      ]
    })
  ]
}
