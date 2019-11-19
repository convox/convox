package azure

func (p *Provider) Heartbeat() (map[string]interface{}, error) {
	hs := map[string]interface{}{
		"region": p.Region,
	}

	return hs, nil
}
