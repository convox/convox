package oci

import "strings"

func (p *Provider) SystemHost() string {
	return p.Domain
}

func (p *Provider) SystemStatus() (string, error) {
	return "running", nil
}

// GPUIntanceList returns the subset of the given OCI shapes that have NVIDIA GPUs.
// OCI GPU shapes are named VM.GPU* and BM.GPU*; the BM.GPU.MI* families are AMD
// Instinct, so they are excluded.
func (p *Provider) GPUIntanceList(instanceTypes []string) ([]string, error) {
	results := []string{}
	for _, instanceType := range instanceTypes {
		upper := strings.ToUpper(instanceType)
		if strings.HasPrefix(upper, "BM.GPU.MI") {
			continue
		}
		if strings.HasPrefix(upper, "VM.GPU") || strings.HasPrefix(upper, "BM.GPU") {
			results = append(results, instanceType)
		}
	}
	return results, nil
}
