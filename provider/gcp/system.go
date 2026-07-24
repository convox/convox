package gcp

import "strings"

func (p *Provider) SystemHost() string {
	return p.Domain
}

func (p *Provider) SystemStatus() (string, error) {
	return "running", nil
}

// gcpGpuMachinePrefixes are GKE machine families that ship with attached NVIDIA GPUs:
// g2 (L4), a2 (A100), a3 (H100), a4/a4x (B200, GB200), g4 (RTX PRO 6000). N1 can attach
// GPUs by choice and is not name-detectable, so it is intentionally not matched here.
var gcpGpuMachinePrefixes = []string{"g2-", "a2-", "a3-", "a4", "g4-"}

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
