package azure

import "strings"

func (p *Provider) SystemHost() string {
	return p.Domain
}

func (p *Provider) SystemStatus() (string, error) {
	return "running", nil
}

// GPUIntanceList returns the subset of the given instance types that have NVIDIA GPUs.
// Azure GPU VM families: Standard_NC* (compute), Standard_ND* (deep learning), Standard_NV* (visualization).
func (p *Provider) GPUIntanceList(instanceTypes []string) ([]string, error) {
	results := []string{}
	for _, instanceType := range instanceTypes {
		upper := strings.ToUpper(instanceType)
		if strings.HasPrefix(upper, "STANDARD_NC") ||
			strings.HasPrefix(upper, "STANDARD_ND") ||
			strings.HasPrefix(upper, "STANDARD_NV") {
			results = append(results, instanceType)
		}
	}
	return results, nil
}
