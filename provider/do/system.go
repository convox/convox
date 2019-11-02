package do

func (p *Provider) SystemHost() string {
	return p.Domain
}

func (p *Provider) SystemStatus() (string, error) {
	return "running", nil
}
