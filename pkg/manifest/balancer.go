package manifest

type Balancer struct {
	Name string `yaml:"-"`

	Ports   BalancerPorts `yaml:"ports,omitempty"`
	Service string        `yaml:"service,omitempty"`
}

type Balancers []Balancer

type BalancerPort struct {
	Source int `yaml:"-"`

	Protocol string `yaml:"protocol,omitempty"`
	Target   int    `yaml:"port,omitempty"`
}

type BalancerPorts []BalancerPort
