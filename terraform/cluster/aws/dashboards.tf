# GPU observability dashboard ConfigMaps.
#
# These ConfigMaps carry the `grafana_dashboard=1` label so a Grafana
# instance with sidecar discovery (a user's BYO Grafana with matching
# sidecar config) auto-imports the dashboards.
#
# Users running the standalone Grafana convox app at
# convox-examples/grafana-gpu-dashboards do not consume these ConfigMaps —
# that app uses file-based provisioning via configMounts. The ConfigMaps
# are inert decoration in that path; ~6 KiB × 6 files = ~36 KiB total, no
# operational cost.
#
# Namespace = kube-system (matches existing dcgm.tf ConfigMap pattern). The
# convox-monitoring namespace is created at runtime by the in-cluster
# control plane, NOT by rack TF, so deploying ConfigMaps to it would fail
# when gpu_observability_enable=true on a rack that has not yet had paid
# metrics enabled. kube-system exists at TF apply time (cluster bootstrap)
# regardless of monitoring state.
#
# Per-dashboard ConfigMap (NOT all-in-one) for independent updates and
# traceable resource naming. Sourced from terraform/cluster/aws/dashboards/
# at TF apply time; that directory is populated by `make sync-dashboards`
# from the source-of-truth at examples/gpu-llm/grafana/.
#
# Gated on var.gpu_observability_enable; ConfigMaps deploy when GPU
# observability is on regardless of in-cluster Grafana state — dashboards
# remain deployable whether the rack runs paid metrics or the free
# lightweight chart or neither.

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
