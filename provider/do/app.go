package do

func (p *Provider) AppIdles(name string) (bool, error) {
	return false, nil
}

func (p *Provider) AppParameters() map[string]string {
	return map[string]string{}
}

func (p *Provider) AppStatus(name string) (string, error) {
	return "running", nil
}
