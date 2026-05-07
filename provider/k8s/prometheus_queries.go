// Package k8s — PromQL source-of-truth constants for GPU observability dashboards.
//
// All Convox-authored PromQL strings used by the GPU dashboards live here. Two
// downstream consumers read these strings:
//
//  1. Rack PromQL methods (QueryGPUMetrics, QueryClusterGpuMetrics,
//     QueryGpuHealthMetrics, QueryKarpenterMetrics) issue these queries
//     against the cluster's Prometheus.
//  2. JSON dashboards under examples/gpu-llm/grafana/*.json carry the
//     identical PromQL strings on each panel target, annotated with
//     `_source: "<CONST_NAME>"`. Bidirectional parity tests in
//     prometheus_parity_test.go enforce alignment.
//
// The third consumer is examples/gpu-llm/grafana/promql-source-of-truth.yaml,
// hand-mirrored from this file; the parity test catches drift.
//
// LABEL CONVENTION (verified live against rack test-v3-3246rc11 2026-05-06):
//
//	DCGM-source consts use the BARE pod-label form (no `label_` prefix):
//
//	  DCGM_FI_DEV_GPU_UTIL{namespace="...", app="...", service="...", pod="..."}
//
//	The dcgm-exporter chart at version 4.8.1 with kubernetes.enablePodLabels=true
//	surfaces K8s pod labels DIRECTLY as Prometheus labels. The `label_` prefix is
//	a kube-state-metrics convention, not a DCGM one. Filter constants targeting
//	DCGM_FI_* metrics use `{namespace=~"$namespace"}` (per-app dashboards) or no
//	filter at all (cluster-aggregate dashboards).
//
//	KSM-source consts (kube_pod_*, kube_deployment_*) emit pod-label rows as
//	`kube_pod_labels{label_app="...", label_service="..."}` — this IS the KSM
//	pod-label injection convention. Cross-metric joins of the form
//
//	  <metric> * on (namespace,pod) group_left(label_app,label_service) kube_pod_labels
//
//	are how D3 panel 5 / replica-count panels filter Convox apps.
//
//	HTTP RED metrics (http_requests_total, http_request_duration_seconds) come
//	from user instrumentation (FastAPI, vLLM, etc.) and use bare K8s
//	`namespace=`/`service=` labels directly — neither DCGM-style nor KSM-style.
//
// CI parity: the `--metric-labels-allowlist` arg on KSM is a hard prerequisite
// for the KSM-source consts to populate. Without it, kube_pod_labels has zero
// series and every join returns empty.
//
// SECTION markers below partition consts by category for merge-conflict
// prevention when multiple workstreams touch this file.
package k8s

const (
	// ===== SECTION: Per-pod DCGM (D2) =====
	// DCGM-source. Bare label form. Filter by Grafana template var $namespace
	// which maps to the convox K8s namespace (e.g. test-v3-rack-app-name).
	GpuTensorActiveByPod = `topk(10, avg by (pod) (DCGM_FI_PROF_PIPE_TENSOR_ACTIVE{namespace=~"$namespace"}) * 100)`
	GpuSmActiveByPod     = `topk(10, avg by (pod) (DCGM_FI_PROF_SM_ACTIVE{namespace=~"$namespace"}) * 100)`
	GpuDramActiveByPod   = `avg by (pod) (DCGM_FI_PROF_DRAM_ACTIVE{namespace=~"$namespace"}) * 100`
	GpuFp16ActiveByPod   = `avg by (pod) (DCGM_FI_PROF_PIPE_FP16_ACTIVE{namespace=~"$namespace"})`
	GpuFp32ActiveByPod   = `avg by (pod) (DCGM_FI_PROF_PIPE_FP32_ACTIVE{namespace=~"$namespace"})`
	GpuFp64ActiveByPod   = `avg by (pod) (DCGM_FI_PROF_PIPE_FP64_ACTIVE{namespace=~"$namespace"})`
	GpuVramUsedByPod     = `sum by (pod) (DCGM_FI_DEV_FB_USED{namespace=~"$namespace"})`
	GpuVramFreeByPod     = `sum by (pod) (DCGM_FI_DEV_FB_FREE{namespace=~"$namespace"})`
	GpuPowerByPod        = `sum by (pod) (DCGM_FI_DEV_POWER_USAGE{namespace=~"$namespace"})`
	// ===== END SECTION: Per-pod DCGM =====

	// ===== SECTION: Cluster-aggregate DCGM (D1) =====
	// DCGM-source. No label filter — cluster-wide aggregate.
	GpuTensorActiveCluster = `avg(DCGM_FI_PROF_PIPE_TENSOR_ACTIVE) * 100`
	GpuSmActiveCluster     = `avg(DCGM_FI_PROF_SM_ACTIVE) * 100`
	GpuDramActiveCluster   = `avg(DCGM_FI_PROF_DRAM_ACTIVE) * 100`
	GpuUtilSanityCluster   = `avg(DCGM_FI_DEV_GPU_UTIL)`
	GpuTotalPowerCluster   = `sum(DCGM_FI_DEV_POWER_USAGE)`
	GpuAllocatedCount      = `sum(DCGM_FI_DEV_FB_USED > 0)`
	GpuTotalCount          = `count(DCGM_FI_DEV_FB_FREE)`
	GpuTopThrottleReasons  = `topk(5, max_over_time((DCGM_FI_DEV_CLOCKS_EVENT_REASONS != 0)[5m:30s])) by (UUID, Hostname)`
	GpuTempHeatmap         = `DCGM_FI_DEV_GPU_TEMP`
	// ===== END SECTION: Cluster-aggregate DCGM =====

	// ===== SECTION: Health per-GPU DCGM (D5) =====
	// DCGM-source. Per-GPU aggregation — no $namespace filter (operator view,
	// not app-scoped). The clock-event-reasons bitmask carries the same data
	// the field formerly named DCGM_FI_DEV_CLOCK_THROTTLE_REASONS held;
	// NVIDIA renamed it in the DCGM 4.x C library, and the dcgm-exporter
	// counters file rejects the deprecated alias.
	GpuXidErrorRateByGpu    = `sum by (UUID, Hostname) (rate(DCGM_FI_DEV_XID_ERRORS[5m]))`
	GpuEccDbeTotalByGpu     = `DCGM_FI_DEV_ECC_DBE_VOL_TOTAL`
	GpuEccSbeRateByGpu      = `sum by (UUID) (rate(DCGM_FI_DEV_ECC_SBE_VOL_TOTAL[1h]))`
	GpuNvlinkReplayByGpu    = `sum by (UUID, link) (rate(DCGM_FI_DEV_NVLINK_REPLAY_ERROR_COUNT_TOTAL[5m]))`
	GpuTempByGpu            = `DCGM_FI_DEV_GPU_TEMP`
	GpuMemTempByGpu         = `DCGM_FI_DEV_MEMORY_TEMP`
	GpuPowerByGpu           = `DCGM_FI_DEV_POWER_USAGE`
	GpuThrottleBitmaskByGpu = `DCGM_FI_DEV_CLOCKS_EVENT_REASONS`
	// ===== END SECTION: Health per-GPU DCGM =====

	// ===== SECTION: HTTP RED + KSM (D3) =====
	// HTTP RED metrics use bare K8s `namespace=`/`service=` labels (user
	// instrumentation convention).
	HttpRequestRateByService   = `sum(rate(http_requests_total{namespace=~"$namespace",service=~"$service"}[5m]))`
	HttpErrorRateByStatusClass = `sum by (status_code) (rate(http_requests_total{namespace=~"$namespace",service=~"$service",status_code=~"4..|5.."}[5m]))`
	HttpLatencyP50ByService    = `histogram_quantile(0.5, sum by (le) (rate(http_request_duration_seconds_bucket{namespace=~"$namespace"}[5m])))`
	HttpLatencyP95ByService    = `histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{namespace=~"$namespace"}[5m])))`
	HttpLatencyP99ByService    = `histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{namespace=~"$namespace"}[5m])))`
	HttpPendingByService       = `sum(http_requests_pending{namespace=~"$namespace",service=~"$service"}) or vector(0)`
	// KSM-source. kube_pod_status_phase carries bare `namespace=`/`pod=` labels
	// (KSM resource labels). The `label_app`/`label_service` cross-join via
	// kube_pod_labels requires --metric-labels-allowlist on KSM.
	K8sReplicaCountByService = `sum(kube_pod_status_phase{namespace=~"$namespace",phase="Running",pod=~"$service-.*"})`
	K8sRestartsByService     = `sum(increase(kube_pod_container_status_restarts_total{namespace=~"$namespace"}[1h]))`
	// ===== END SECTION: HTTP RED + KSM =====

	// ===== SECTION: Karpenter (D7) =====
	// Karpenter metrics are cluster-scoped (no $namespace var). GPU NodePools
	// are filtered by the `nodepool` label substring `gpu` per Convox naming
	// convention.
	KarpenterGpuNodeCount         = `sum by (nodepool) (karpenter_nodes_total_count{nodepool=~".*gpu.*"})`
	KarpenterGpuClaimRate         = `sum by (nodepool) (rate(karpenter_nodeclaims_created_total{nodepool=~".*gpu.*"}[10m]))`
	KarpenterPodPendingDuration   = `karpenter_pods_unbound_time_seconds`
	KarpenterSpotInterruptionRate = `sum by (message_type) (rate(karpenter_interruption_received_messages_total[1h]))`
	KarpenterDisruptionRate       = `sum by (reason) (rate(karpenter_voluntary_disruption_decisions_total[1h]))`
	KarpenterNodeLifetime         = `karpenter_nodes_lifetime_seconds{nodepool=~".*gpu.*"}`
	KarpenterNodepoolUsage        = `karpenter_nodepool_usage{nodepool=~".*gpu.*"}`
	KarpenterNodepoolLimit        = `karpenter_nodepool_limit{nodepool=~".*gpu.*"}`
	// ===== END SECTION: Karpenter =====

	// ===== SECTION: vLLM (D6 upstream JSON; convox-authored extensions) =====
	// D6 base panels are upstream-exempt via `convox_source_check:upstream` top-
	// level tag on the JSON; the parity test skips forward checks against
	// upstream-tagged files. Convox-authored extension panels (KV-cache
	// pressure current/trend) carry their own `_source: "convox-authored:..."`
	// annotations and do NOT need a Go const (the convox-authored: prefix is a
	// recognized parity exemption). Future stronger-parity extensions can add
	// consts here.
	// ===== END SECTION: vLLM =====
)

// AllPromQLConstants returns every Convox-authored PromQL string keyed by Go
// const identifier. Bidirectional parity tests enumerate this map:
//
//   - TestPromQLParityGoVsYaml: every entry exists in the YAML manifest with the
//     same trimmed value; the YAML doesn't list unknown keys.
//
//   - TestPromQLConstantsAppearInJsonDashboards: forward parity — for every JSON
//     dashboard target carrying `_source: "<CONST_NAME>"`, the panel's `expr`
//     equals the const value verbatim.
//
//   - TestEveryPanelTargetHasSource: reverse parity — every JSON panel target
//     carries an `_source` annotation that is either a Go const name, an
//     `upstream:*` exemption tag, or a `convox-authored:*` extension tag.
func AllPromQLConstants() map[string]string {
	return map[string]string{
		// Per-pod DCGM (D2)
		"GpuTensorActiveByPod": GpuTensorActiveByPod,
		"GpuSmActiveByPod":     GpuSmActiveByPod,
		"GpuDramActiveByPod":   GpuDramActiveByPod,
		"GpuFp16ActiveByPod":   GpuFp16ActiveByPod,
		"GpuFp32ActiveByPod":   GpuFp32ActiveByPod,
		"GpuFp64ActiveByPod":   GpuFp64ActiveByPod,
		"GpuVramUsedByPod":     GpuVramUsedByPod,
		"GpuVramFreeByPod":     GpuVramFreeByPod,
		"GpuPowerByPod":        GpuPowerByPod,
		// Cluster-aggregate DCGM (D1)
		"GpuTensorActiveCluster": GpuTensorActiveCluster,
		"GpuSmActiveCluster":     GpuSmActiveCluster,
		"GpuDramActiveCluster":   GpuDramActiveCluster,
		"GpuUtilSanityCluster":   GpuUtilSanityCluster,
		"GpuTotalPowerCluster":   GpuTotalPowerCluster,
		"GpuAllocatedCount":      GpuAllocatedCount,
		"GpuTotalCount":          GpuTotalCount,
		"GpuTopThrottleReasons":  GpuTopThrottleReasons,
		"GpuTempHeatmap":         GpuTempHeatmap,
		// Health per-GPU DCGM (D5)
		"GpuXidErrorRateByGpu":    GpuXidErrorRateByGpu,
		"GpuEccDbeTotalByGpu":     GpuEccDbeTotalByGpu,
		"GpuEccSbeRateByGpu":      GpuEccSbeRateByGpu,
		"GpuNvlinkReplayByGpu":    GpuNvlinkReplayByGpu,
		"GpuTempByGpu":            GpuTempByGpu,
		"GpuMemTempByGpu":         GpuMemTempByGpu,
		"GpuPowerByGpu":           GpuPowerByGpu,
		"GpuThrottleBitmaskByGpu": GpuThrottleBitmaskByGpu,
		// HTTP RED + KSM (D3)
		"HttpRequestRateByService":   HttpRequestRateByService,
		"HttpErrorRateByStatusClass": HttpErrorRateByStatusClass,
		"HttpLatencyP50ByService":    HttpLatencyP50ByService,
		"HttpLatencyP95ByService":    HttpLatencyP95ByService,
		"HttpLatencyP99ByService":    HttpLatencyP99ByService,
		"HttpPendingByService":       HttpPendingByService,
		"K8sReplicaCountByService":   K8sReplicaCountByService,
		"K8sRestartsByService":       K8sRestartsByService,
		// Karpenter (D7)
		"KarpenterGpuNodeCount":         KarpenterGpuNodeCount,
		"KarpenterGpuClaimRate":         KarpenterGpuClaimRate,
		"KarpenterPodPendingDuration":   KarpenterPodPendingDuration,
		"KarpenterSpotInterruptionRate": KarpenterSpotInterruptionRate,
		"KarpenterDisruptionRate":       KarpenterDisruptionRate,
		"KarpenterNodeLifetime":         KarpenterNodeLifetime,
		"KarpenterNodepoolUsage":        KarpenterNodepoolUsage,
		"KarpenterNodepoolLimit":        KarpenterNodepoolLimit,
	}
}

// dcgmSourceConsts lists Go const names whose PromQL strings target DCGM_FI_*
// metrics. Used by TestPromQLConstantsHaveLabelFilters to apply the bare-label
// regex (DCGM convention) instead of the `label_*` regex (KSM convention).
//
// Cluster-aggregate consts have no label filter and are excluded from the
// per-pod app-filter check entirely.
func dcgmPerPodSourceConsts() []string {
	return []string{
		"GpuTensorActiveByPod",
		"GpuSmActiveByPod",
		"GpuDramActiveByPod",
		"GpuFp16ActiveByPod",
		"GpuFp32ActiveByPod",
		"GpuFp64ActiveByPod",
		"GpuVramUsedByPod",
		"GpuVramFreeByPod",
		"GpuPowerByPod",
	}
}

// httpRedSourceConsts lists Go const names whose PromQL strings target user
// instrumentation HTTP RED metrics. These use bare K8s `namespace=`/`service=`
// labels.
func httpRedSourceConsts() []string {
	return []string{
		"HttpRequestRateByService",
		"HttpErrorRateByStatusClass",
		"HttpLatencyP50ByService",
		"HttpLatencyP95ByService",
		"HttpLatencyP99ByService",
		"HttpPendingByService",
	}
}
