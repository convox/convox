# Free-path GPU-metrics Prometheus.
# prometheus-community/prometheus chart, scoped to scraping DCGM only.
# Subcomponents disabled. Chart ships ZERO CRDs and ZERO webhooks.
#
# Gated on gpu_observability_enable=true AND monitoring_metrics_provisioned=false.
# Console3 mutation flips monitoring_metrics_provisioned=true after paid install.

resource "helm_release" "prometheus_gpu_metrics" {
  depends_on = [
    null_resource.wait_k8s_api,
    helm_release.dcgm_exporter,
  ]

  count = var.gpu_observability_enable && !var.monitoring_metrics_provisioned ? 1 : 0

  name       = "prometheus-gpu-metrics"
  repository = "https://prometheus-community.github.io/helm-charts"
  chart      = "prometheus"
  version    = var.prometheus_gpu_metrics_chart_version
  namespace  = "kube-system"

  values = [
    yamlencode({
      server = {
        global = {
          scrape_interval = "30s"
        }
        persistentVolume = { enabled = false }
        retention        = var.prometheus_gpu_metrics_retention
        resources = {
          requests = { cpu = "100m", memory = "512Mi" }
          limits   = { cpu = "500m", memory = "1024Mi" }
        }
      }
      alertmanager               = { enabled = false }
      "prometheus-pushgateway"   = { enabled = false }
      "prometheus-node-exporter" = { enabled = false }
      "kube-state-metrics"       = { enabled = false }
      # serverFiles."prometheus.yml".scrape_configs uses Helm's array-replacement merge:
      # the user-provided list REPLACES the chart's default scrape_configs entirely.
      # NOTE: do NOT set `global` here — chart already emits `global:` from server.global.
      # DCGM scrape config sourced from shared YAML asset (single source of truth
      # consumed by both this rack TF and Console3 helm values; see SPEC §3.25).
      serverFiles = {
        "prometheus.yml" = {
          scrape_configs = [
            {
              job_name     = "prometheus"
              metrics_path = "/metrics"
              static_configs = [
                { targets = ["localhost:9090"] },
              ]
            },
            local.dcgm_scrape_config,
          ]
        }
      }
    })
  ]
}

locals {
  # THREE ".." hops: aws/ -> cluster/ -> terraform/ -> repo-root.
  # Companion go:embed lives at pkg/structs/dcgm_scrape.go.
  dcgm_scrape_config = yamldecode(file("${path.module}/../../../pkg/structs/dcgm-scrape.yaml"))
}
