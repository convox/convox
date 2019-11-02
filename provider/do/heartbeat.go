package do

func (p *Provider) Heartbeat() (map[string]interface{}, error) {
	hs := map[string]interface{}{
		"instance_type": "unknown",
		"region":        p.Region,
	}

	return hs, nil
}
