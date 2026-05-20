// Label conventions: DCGM metrics use bare pod labels; KSM metrics use `label_` prefix;
// HTTP RED metrics use bare K8s namespace/service labels. KSM requires --metric-labels-allowlist.
//
// SECTION markers below partition consts by category for merge-conflict
// prevention when multiple workstreams touch this file.
package k8s

const (
	// ===== SECTION: Per-pod DCGM =====
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

	// ===== SECTION: Cluster-aggregate DCGM =====
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

	// ===== SECTION: Health per-GPU DCGM =====
	GpuXidErrorRateByGpu    = `sum by (UUID, Hostname) (rate(DCGM_FI_DEV_XID_ERRORS[5m]))`
	GpuEccDbeTotalByGpu     = `DCGM_FI_DEV_ECC_DBE_VOL_TOTAL`
	GpuEccSbeRateByGpu      = `sum by (UUID) (rate(DCGM_FI_DEV_ECC_SBE_VOL_TOTAL[1h]))`
	GpuNvlinkReplayByGpu    = `sum by (UUID, link) (rate(DCGM_FI_DEV_NVLINK_REPLAY_ERROR_COUNT_TOTAL[5m]))`
	GpuTempByGpu            = `DCGM_FI_DEV_GPU_TEMP`
	GpuMemTempByGpu         = `DCGM_FI_DEV_MEMORY_TEMP`
	GpuPowerByGpu           = `DCGM_FI_DEV_POWER_USAGE`
	GpuThrottleBitmaskByGpu = `DCGM_FI_DEV_CLOCKS_EVENT_REASONS`
	// ===== END SECTION: Health per-GPU DCGM =====

	// ===== SECTION: HTTP RED + KSM =====
	HttpRequestRateByService   = `sum(rate(http_requests_total{namespace=~"$namespace",service=~"$service"}[5m]))`
	HttpErrorRateByStatusClass = `sum by (status_code) (rate(http_requests_total{namespace=~"$namespace",service=~"$service",status_code=~"4..|5.."}[5m]))`
	HttpLatencyP50ByService    = `histogram_quantile(0.5, sum by (le) (rate(http_request_duration_seconds_bucket{namespace=~"$namespace"}[5m])))`
	HttpLatencyP95ByService    = `histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket{namespace=~"$namespace"}[5m])))`
	HttpLatencyP99ByService    = `histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{namespace=~"$namespace"}[5m])))`
	HttpPendingByService       = `sum(http_requests_pending{namespace=~"$namespace",service=~"$service"}) or vector(0)`
	K8sReplicaCountByService   = `sum(kube_pod_status_phase{namespace=~"$namespace",phase="Running",pod=~"$service-.*"})`
	K8sRestartsByService       = `sum(increase(kube_pod_container_status_restarts_total{namespace=~"$namespace"}[1h]))`
	// ===== END SECTION: HTTP RED + KSM =====

	// ===== SECTION: Karpenter =====
	KarpenterGpuNodeCount         = `sum by (nodepool) (karpenter_nodes_total_count{nodepool=~".*gpu.*"})`
	KarpenterGpuClaimRate         = `sum by (nodepool) (rate(karpenter_nodeclaims_created_total{nodepool=~".*gpu.*"}[10m]))`
	KarpenterPodPendingDuration   = `karpenter_pods_unbound_time_seconds`
	KarpenterSpotInterruptionRate = `sum by (message_type) (rate(karpenter_interruption_received_messages_total[1h]))`
	KarpenterDisruptionRate       = `sum by (reason) (rate(karpenter_voluntary_disruption_decisions_total[1h]))`
	KarpenterNodeLifetime         = `karpenter_nodes_lifetime_seconds{nodepool=~".*gpu.*"}`
	KarpenterNodepoolUsage        = `karpenter_nodepool_usage{nodepool=~".*gpu.*"}`
	KarpenterNodepoolLimit        = `karpenter_nodepool_limit{nodepool=~".*gpu.*"}`
	// ===== END SECTION: Karpenter =====

	// ===== SECTION: vLLM =====
	// ===== END SECTION: vLLM =====
)

func AllPromQLConstants() map[string]string {
	return map[string]string{
		"GpuTensorActiveByPod":          GpuTensorActiveByPod,
		"GpuSmActiveByPod":              GpuSmActiveByPod,
		"GpuDramActiveByPod":            GpuDramActiveByPod,
		"GpuFp16ActiveByPod":            GpuFp16ActiveByPod,
		"GpuFp32ActiveByPod":            GpuFp32ActiveByPod,
		"GpuFp64ActiveByPod":            GpuFp64ActiveByPod,
		"GpuVramUsedByPod":              GpuVramUsedByPod,
		"GpuVramFreeByPod":              GpuVramFreeByPod,
		"GpuPowerByPod":                 GpuPowerByPod,
		"GpuTensorActiveCluster":        GpuTensorActiveCluster,
		"GpuSmActiveCluster":            GpuSmActiveCluster,
		"GpuDramActiveCluster":          GpuDramActiveCluster,
		"GpuUtilSanityCluster":          GpuUtilSanityCluster,
		"GpuTotalPowerCluster":          GpuTotalPowerCluster,
		"GpuAllocatedCount":             GpuAllocatedCount,
		"GpuTotalCount":                 GpuTotalCount,
		"GpuTopThrottleReasons":         GpuTopThrottleReasons,
		"GpuTempHeatmap":                GpuTempHeatmap,
		"GpuXidErrorRateByGpu":          GpuXidErrorRateByGpu,
		"GpuEccDbeTotalByGpu":           GpuEccDbeTotalByGpu,
		"GpuEccSbeRateByGpu":            GpuEccSbeRateByGpu,
		"GpuNvlinkReplayByGpu":          GpuNvlinkReplayByGpu,
		"GpuTempByGpu":                  GpuTempByGpu,
		"GpuMemTempByGpu":               GpuMemTempByGpu,
		"GpuPowerByGpu":                 GpuPowerByGpu,
		"GpuThrottleBitmaskByGpu":       GpuThrottleBitmaskByGpu,
		"HttpRequestRateByService":      HttpRequestRateByService,
		"HttpErrorRateByStatusClass":    HttpErrorRateByStatusClass,
		"HttpLatencyP50ByService":       HttpLatencyP50ByService,
		"HttpLatencyP95ByService":       HttpLatencyP95ByService,
		"HttpLatencyP99ByService":       HttpLatencyP99ByService,
		"HttpPendingByService":          HttpPendingByService,
		"K8sReplicaCountByService":      K8sReplicaCountByService,
		"K8sRestartsByService":          K8sRestartsByService,
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
