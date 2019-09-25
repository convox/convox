package gcp

import (
	"io/ioutil"
	"net/http"
	"strings"
)

func (p *Provider) Heartbeat() (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/machine-type", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Metadata-Flavor", "Google")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	tparts := strings.Split(strings.TrimSpace(string(data)), "/")

	hs := map[string]interface{}{
		"instance_type": tparts[len(tparts)-1],
		"region":        p.Region,
	}

	return hs, nil
}
