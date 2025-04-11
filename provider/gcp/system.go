package gcp

func (p *Provider) SystemHost() string {
	return p.Domain
}

func (p *Provider) SystemStatus() (string, error) {
	return "running", nil
}

func (p *Provider) GPUIntanceList(instanceTypes []string) ([]string, error) {
	return nil, nil
}
