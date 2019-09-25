package aws

import (
	"strings"

	"github.com/convox/convox/pkg/common"
)

func (p *Provider) Heartbeat() (map[string]interface{}, error) {
	data, err := common.Get("http://169.254.169.254/latest/meta-data/instance-type")
	if err != nil {
		return nil, err
	}

	hs := map[string]interface{}{
		"instance_type": strings.TrimSpace(string(data)),
		"region":        p.Region,
	}

	return hs, nil
}
