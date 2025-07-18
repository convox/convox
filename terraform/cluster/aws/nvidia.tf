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

  values = [
    yamlencode({
      config = {
        map = var.nvidia_device_time_slicing_replicas <= 1 ? {} : {
          default = yamlencode({
            version = "v1"
            flags = {
              "migStrategy" : "none"
            }
            sharing = {
              timeSlicing = {
                resources = [{
                  name     = "nvidia.com/gpu"
                  replicas = var.nvidia_device_time_slicing_replicas
                }]
              }
            }
          })
        }
      }
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
        }
      ]
    })
  ]
}
