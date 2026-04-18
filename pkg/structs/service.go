package structs

type Service struct {
	Count     int           `json:"count"`
	Cpu       int           `json:"cpu"`
	Domain    string        `json:"domain"`
	Gpu       int           `json:"gpu"`
	GpuVendor string        `json:"gpu-vendor"`
	Memory    int           `json:"memory"`
	Name      string        `json:"name"`
	Ports     []ServicePort `json:"ports"`
}

type Services []Service

type ServicePort struct {
	Balancer    int    `json:"balancer"`
	Certificate string `json:"certificate"`
	Container   int    `json:"container"`
}

type ServiceUpdateOptions struct {
	Count     *int    `flag:"count" param:"count"`
	Cpu       *int    `flag:"cpu" param:"cpu"`
	Gpu       *int    `flag:"gpu" param:"gpu"`
	GpuVendor *string `flag:"gpu-vendor" param:"gpu-vendor"`
	Memory    *int    `flag:"memory" param:"memory"`
}
