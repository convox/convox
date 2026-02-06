resource "helm_release" "vpa" {
  count            = var.vpa_enable ? 1 : 0
  name             = "vpa"
  repository       = "https://charts.fairwinds.com/stable"
  chart            = "vpa"
  version          = "4.10.1"
  namespace        = "vpa"
  create_namespace = true

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
