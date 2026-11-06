package manifest

import "strings"

const (
	BalancerProtocolTcp    = "TCP"
	BalancerProtocolTcpUdp = "TCP_UDP"
	BalancerProtocolUdp    = "UDP"
)

type Balancer struct {
	Name string `yaml:"-"`

	Annotations               BalancerAnnotations `yaml:"annotations"`
	AwsLoadBalancerController bool                `yaml:"awsLoadBalancerController,omitempty"`
	Ports                     BalancerPorts       `yaml:"ports,omitempty"`
	Service                   string              `yaml:"service,omitempty"`
	Whitelist                 BalancerWhitelist   `yaml:"whitelist,omitempty"`
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
		if len(parts) != 2 {
			continue
		}
		annotations[parts[0]] = parts[1]
	}

	return annotations
}
