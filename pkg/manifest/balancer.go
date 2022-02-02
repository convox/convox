package manifest

import "strings"

type Balancer struct {
	Name string `yaml:"-"`

	Annotations BalancerAnnotations `yaml:"annotations"`
	Ports       BalancerPorts       `yaml:"ports,omitempty"`
	Service     string              `yaml:"service,omitempty"`
	Whitelist   BalancerWhitelist   `yaml:"whitelist,omitempty"`
}

type Balancers []Balancer

type BalancerAnnotations []string

type BalancerPort struct {
	Source int `yaml:"-"`

	Protocol string `yaml:"protocol,omitempty"`
	Target   int    `yaml:"port,omitempty"`
}

type BalancerPorts []BalancerPort

type BalancerWhitelist []string

func (b Balancer) AnnotationsMap() map[string]string {
	annotations := map[string]string{}

	for _, a := range b.Annotations {
		parts := strings.SplitN(a, "=", 2)
		annotations[parts[0]] = parts[1]
	}

	return annotations
}
