# GPU observability dashboard ConfigMaps (one per dashboard for independent updates).
# Labeled grafana_dashboard=1 for sidecar discovery by BYO Grafana instances.
# Namespace = kube-system (convox-monitoring may not exist at TF apply time).
# Gated on var.gpu_observability_enable.

resource "kubernetes_config_map" "gpu_dashboards" {
  for_each = var.gpu_observability_enable ? fileset("${path.module}/dashboards", "*.json") : toset([])

  metadata {
    name      = "convox-gpu-dashboard-${replace(replace(each.key, ".json", ""), "_", "-")}"
    namespace = "kube-system"
    labels = {
      "grafana_dashboard"            = "1"
      "app.kubernetes.io/part-of"    = "convox-gpu-observability"
      "app.kubernetes.io/managed-by" = "convox"
    }
  }

  data = {
    "${each.key}" = file("${path.module}/dashboards/${each.key}")
  }
}
