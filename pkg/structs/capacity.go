package structs

type Capacity struct {
	ClusterCPU    int64 `json:"cluster-cpu"`
	ClusterGPU    int64 `json:"cluster-gpu"`
	ClusterMemory int64 `json:"cluster-memory"`
	ProcessCount  int64 `json:"process-count"`
	ProcessCPU    int64 `json:"process-cpu"`
	ProcessGPU    int64 `json:"process-gpu"`
	ProcessMemory int64 `json:"process-memory"`
}
