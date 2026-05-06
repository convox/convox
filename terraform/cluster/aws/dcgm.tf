# DCGM exporter — emits NVIDIA GPU utilization, VRAM, temp, power metrics
# on port 9400. Source of truth for the GPU observability slice.
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
#
# DCGM counter override: chart 4.8.1's stock default-counters.csv leaves XID,
# ECC SBE/DBE, NVLINK_REPLAY, CLOCK_THROTTLE_REASONS, SM_ACTIVE, FP16/32/64
# ACTIVE disabled. files/dcp-metrics-included.csv (this directory) ships them
# enabled; the ConfigMap + extraConfigMapVolumes/extraVolumeMounts pattern
# below mounts the file at /etc/dcgm-exporter-convox/ and points the
# exporter's -f arg there. Verified upstream chart 4.8.1 supports the keys
# (deployment/templates/daemonset.yaml `{{- with .Values.extraConfigMapVolumes }}`).

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
    null_resource.wait_k8s_api,
    helm_release.nvidia_device_plugin,
    kubernetes_config_map.dcgm_metrics_convox,
  ]

  count = var.gpu_observability_enable && var.nvidia_device_plugin_enable ? 1 : 0

  name       = "dcgm-exporter"
  repository = "https://nvidia.github.io/dcgm-exporter/helm-charts"
  chart      = "dcgm-exporter"
  # MAINTENANCE: literal default MUST match terraform/cluster/aws/variables.tf
  # default for gpu_observability_chart_version. Empty value falls through so
  # the user can clear the override; coalesce keeps helm_release valid.
  version   = coalesce(var.gpu_observability_chart_version, "4.8.1")
  namespace = "kube-system"

  values = [
    yamlencode({
      # Both free path (Convox-Console-installed prometheus-gpu-metrics in
      # kube-system ns, prometheus-community/prometheus chart) and paid path
      # (kube-prometheus-stack in convox-monitoring ns, Convox-Console-managed)
      # discover this exporter via the app.kubernetes.io/name=dcgm-exporter
      # pod label on a kubernetes_sd_configs Pod-role scrape. ServiceMonitor disabled.
      serviceMonitor = {
        enabled = false
      }

      # Pod-attribution: emits "pod" and "namespace" labels on every metric.
      # enablePodLabels=true additionally surfaces each pod's K8s labels
      # DIRECTLY as Prometheus labels — plain `app=`, `service=`, etc. NO
      # `label_` prefix is added by DCGM. (The `label_` prefix is a
      # kube-state-metrics convention, not a DCGM one — confusing the two
      # is what produced the empty-dashboard regression that this comment
      # now exists to prevent.)
      #
      # Convox's Prom queries (provider/k8s/prometheus.go::QueryGPUMetrics
      # + pkg/manifest/service.go KEDA triggers) filter by plain `app=` and
      # `service=`. Load-bearing dependency: the scrape config at
      # pkg/structs/dcgm-scrape.yaml MUST keep `honor_labels: true` so
      # Prometheus does not prepend `exported_` on label collisions.
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

      # Pod annotations preserved for compatibility with user self-installed
      # Prometheus (e.g. kube-prometheus-stack self-managed which expects them).
      # Convox's free + paid paths use kubernetes_sd Pod role + label selector;
      # they do NOT consume these annotations.
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

      # Override stock default-counters.csv with Convox's superset CSV that
      # enables the 9 fields D1/D2/D5 panels need. ConfigMap
      # `convox-dcgm-metrics` is provisioned above (kubernetes_config_map.dcgm_metrics_convox).
      # Mount under a Convox-specific path to avoid colliding with the chart's
      # baked-in default-counters.csv at /etc/dcgm-exporter/.
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

      # Override the default `["-f", "/etc/dcgm-exporter/default-counters.csv"]`
      # to point at Convox's superset file. Chart 4.8.1's daemonset template
      # passes `arguments` verbatim to the container args.
      arguments = ["-f", "/etc/dcgm-exporter-convox/dcp-metrics-included.csv"]
    })
  ]
}
