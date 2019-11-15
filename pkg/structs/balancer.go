package structs

type Balancer struct {
	Name     string        `json:"name"`
	Endpoint string        `json:"endpoint"`
	Ports    BalancerPorts `json:"ports"`
	Service  string        `json:"service"`
}

type Balancers []Balancer

type BalancerPort struct {
	Protocol string `json:"protocol"`
	Source   int    `json:"source"`
	Target   int    `json:"target"`
}

type BalancerPorts []BalancerPort
