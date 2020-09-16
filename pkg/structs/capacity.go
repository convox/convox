package structs

type Capacity struct {
	ClusterCPU    int64 `json:"cluster-cpu"`
	ClusterMemory int64 `json:"cluster-memory"`
	ProcessCount  int64 `json:"process-count"`
	ProcessCPU    int64 `json:"process-cpu"`
	ProcessMemory int64 `json:"process-memory"`
}
