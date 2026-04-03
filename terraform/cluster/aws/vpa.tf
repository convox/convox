resource "helm_release" "vpa" {
  depends_on = [
    null_resource.wait_k8s_api,
  ]

  count            = var.vpa_enable ? 1 : 0
  name             = "vpa"
  repository       = "https://charts.fairwinds.com/stable"
  chart            = "vpa"
  version          = "4.10.1"
  namespace        = "vpa"
  create_namespace = true
  atomic           = true
  timeout          = 600

  values = [
    yamlencode({
      admissionController = {
        extraArgs = {
          feature-gates = "InPlaceOrRecreate=true"
        }
      }

      updater = {
        extraArgs = {
          feature-gates = "InPlaceOrRecreate=true"
        }
      }
    })
  ]
}
