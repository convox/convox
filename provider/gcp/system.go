package gcp

import "strings"

func (p *Provider) SystemHost() string {
	return p.Domain
}

func (p *Provider) SystemStatus() (string, error) {
	return "running", nil
}

// gcpGpuMachinePrefixes are GKE machine families that ship with attached NVIDIA GPUs.
// g2- (L4), a2- (A100), a3- (H100), a4- (B200), g4- (RTX PRO 6000).
// N1 machines can have GPUs attached à la carte, so they cannot be detected by
// machine-type name alone. GKE also stamps GPU nodes with the label
// cloud.google.com/gke-accelerator, but the node controller (controller_node.go)
// already keys off node.kubernetes.io/instance-type, so name-prefix matching here
// stays consistent with that flow without restructuring the controller.
var gcpGpuMachinePrefixes = []string{"g2-", "a2-", "a3-", "a4-", "g4-"}

// GPUIntanceList returns the subset of the given machine types that have NVIDIA GPUs.
func (p *Provider) GPUIntanceList(instanceTypes []string) ([]string, error) {
	results := []string{}
	for _, instanceType := range instanceTypes {
		lower := strings.ToLower(instanceType)
		for _, prefix := range gcpGpuMachinePrefixes {
			if strings.HasPrefix(lower, prefix) {
				results = append(results, instanceType)
				break
			}
		}
	}
	return results, nil
}
