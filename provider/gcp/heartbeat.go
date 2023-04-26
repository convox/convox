package gcp

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	ns, err := p.Cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	spotCnt := 0
	onDemandCnt := 0

	for i := range ns.Items {
		switch ns.Items[i].Labels["cloud.google.com/gke-spot"] {
		case "true":
			spotCnt++
		default:
			onDemandCnt++
		}
	}

	hs := map[string]interface{}{
		"instance_type":            tparts[len(tparts)-1],
		"region":                   p.Region,
		"on_demand_instance_count": onDemandCnt,
		"spot_instance_count":      spotCnt,
	}

	return hs, nil
}
