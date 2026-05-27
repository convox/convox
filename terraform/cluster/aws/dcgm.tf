# DCGM exporter — NVIDIA GPU metrics on port 9400.
# Chart: nvidia/dcgm-exporter (no CRDs, no webhooks, fully reversible).
# Gate: gpu_observability_enable AND nvidia_device_plugin_enable.
# Re-audit on chart major bumps (CRDs/webhooks may appear).
# Custom counter CSV overrides stock defaults via extraConfigMapVolumes.

resource "kubernetes_config_map" "dcgm_metrics_convox" {
  count = var.gpu_observability_enable && var.nvidia_device_plugin_enable ? 1 : 0
  metadata {
    name      = "convox-dcgm-metrics"
    namespace = "kube-system"
    labels = {
      "app.kubernetes.io/managed-by" = "convox"
      "convox.com/role"              = "dcgm-counters"
    }
  }
  data = {
    "dcp-metrics-included.csv" = file("${path.module}/files/dcp-metrics-included.csv")
  }
}

resource "helm_release" "dcgm_exporter" {
  depends_on = [
    null_resource.wait_eks_addons,
    helm_release.nvidia_device_plugin,
    kubernetes_config_map.dcgm_metrics_convox,
  ]

  count = var.gpu_observability_enable && var.nvidia_device_plugin_enable ? 1 : 0

  name       = "dcgm-exporter"
  repository = "https://nvidia.github.io/dcgm-exporter/helm-charts"
  chart      = "dcgm-exporter"
  # Default must match variables.tf; drift asserted by TestCoalesceLiteralsMatchTFDefaults.
  version   = coalesce(var.gpu_observability_chart_version, "4.8.1")
  namespace = "kube-system"

  values = [
    yamlencode({
      # Discovered via pod label scrape, not ServiceMonitor.
      serviceMonitor = {
        enabled = false
      }

      # enablePodLabels surfaces K8s labels as plain Prom labels (no `label_` prefix).
      # Queries filter by `app=` / `service=`; scrape config must keep honor_labels: true.
      kubernetes = {
        enablePodLabels = true
        enablePodUID    = true
        rbac = {
          create = true
        }
      }

      service = {
        enable = true
        type   = "ClusterIP"
        port   = 9400
      }

      # Pod annotations for user-installed Prometheus compatibility.
      # dcgm-csv-sha256: rolling-update trigger when counter CSV changes.
      podAnnotations = {
        "prometheus.io/scrape" = "true"
        "prometheus.io/port"   = "9400"
        "prometheus.io/path"   = "/metrics"
        # Scrape interval hint for user-installed Prometheus via pod annotations.
        "prometheus.io/scrape-interval" = coalesce(var.dcgm_scrape_interval, "15s")
        "convox.com/dcgm-csv-sha256"    = filesha256("${path.module}/files/dcp-metrics-included.csv")
      }

      # Schedule on GPU nodes only (label applied by rack controller at runtime).
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

      # Tolerate GPU and dedicated-node taints (no CriticalAddonsOnly needed).
      tolerations = [
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

      resources = {
        requests = {
          cpu    = "100m"
          memory = "128Mi"
        }
        limits = {
          cpu    = "200m"
          memory = "512Mi"
        }
      }

      # Mount Convox's superset counter CSV; separate path avoids colliding
      # with chart's baked-in default-counters.csv.
      extraConfigMapVolumes = [
        {
          name = "convox-dcgm-metrics"
          configMap = {
            name = "convox-dcgm-metrics"
            items = [
              {
                key  = "dcp-metrics-included.csv"
                path = "dcp-metrics-included.csv"
              },
            ]
          }
        },
      ]

      extraVolumeMounts = [
        {
          name      = "convox-dcgm-metrics"
          mountPath = "/etc/dcgm-exporter-convox"
          readOnly  = true
        },
      ]

      # Point at Convox's superset counter CSV.
      arguments = ["-f", "/etc/dcgm-exporter-convox/dcp-metrics-included.csv"]
    })
  ]
}
