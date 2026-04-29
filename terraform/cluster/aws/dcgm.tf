# DCGM exporter — emits NVIDIA GPU utilization, VRAM, temp, power metrics
# on port 9400. Source of truth for the Plan-5 GPU observability slice.
#
# Chart: NVIDIA upstream dcgm-exporter
# (https://nvidia.github.io/dcgm-exporter/helm-charts).
# Audited 2026-04-28: chart installs ZERO CRDs and ZERO admission webhooks.
# helm uninstall is fully reversible; no orphan resources on disable.
#
# Gate: BOTH var.gpu_observability_enable AND var.nvidia_device_plugin_enable
# must be true. The exporter relies on the device plugin's
# /var/lib/kubelet/pod-resources/ socket for pod->GPU attribution.
#
# When chart major version is bumped (4.x -> 5.x), re-audit per the chart-bump
# checklist in docs/configuration/rack-parameters/aws/gpu_observability_chart_version.md
# before merging — a future major may introduce CRDs / webhooks that require
# a finalizer-cleanup null_resource (mirror karpenter.tf:172-242 pattern).

resource "helm_release" "dcgm_exporter" {
  depends_on = [
    null_resource.wait_k8s_api,
    helm_release.nvidia_device_plugin,
  ]

  count = var.gpu_observability_enable && var.nvidia_device_plugin_enable ? 1 : 0

  name       = "dcgm-exporter"
  repository = "https://nvidia.github.io/dcgm-exporter/helm-charts"
  chart      = "dcgm-exporter"
  version    = var.gpu_observability_chart_version
  namespace  = "kube-system"

  values = [
    yamlencode({
      # Prometheus Operator is NOT installed on Convox racks; disable the
      # ServiceMonitor CR. Plan-5 Prometheus scrape uses kubernetes_sd
      # Pod role + the podAnnotations below instead.
      serviceMonitor = {
        enabled = false
      }

      # Pod-attribution: emits "pod" and "namespace" labels on every metric.
      # enablePodLabels=true additionally surfaces each pod's K8s labels as
      # "label_<key>" metric labels, so item 7's Prom queries can filter
      # by label_app and label_service.
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

      # Scrape annotations consumed by the in-cluster Prometheus job's
      # kubernetes_sd_configs (Pod role) + relabel_configs that keeps
      # targets where __meta_kubernetes_pod_annotation_prometheus_io_scrape
      # == "true".
      podAnnotations = {
        "prometheus.io/scrape" = "true"
        "prometheus.io/port"   = "9400"
        "prometheus.io/path"   = "/metrics"
      }

      # Schedule on GPU nodes only. The convox.io/gpu-vendor=nvidia label
      # is applied at runtime by the rack controller
      # (provider/k8s/controller_node.go:149 AddGpuLabel) when a node's
      # instance type is in the NVIDIA GPU list — eventually-consistent,
      # not TF-driven. Pods will pend until the label appears, which is
      # the desired behavior.
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

      # Tolerate the GPU taint and any custom dedicated-node taints.
      # DCGM is observability — does NOT need CriticalAddonsOnly
      # (which would put it on system nodes).
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
    })
  ]
}
