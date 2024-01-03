package exoscale

import (
	"context"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/pkg/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) Heartbeat() (map[string]interface{}, error) {
	data, err := common.Get("http://169.254.169.254/latest/meta-data/instance-type")
	if err != nil {
		return nil, err
	}

	onDemandCnt, spotCnt, err := p.getInstanceTypeWiseCnt()
	if err != nil {
		return nil, err
	}

	hs := map[string]interface{}{
		"instance_type":            strings.TrimSpace(string(data)),
		"region":                   p.Zone,
		"on_demand_instance_count": onDemandCnt,
		"spot_instance_count":      spotCnt,
	}

	return hs, nil
}

func (p *Provider) getInstanceTypeWiseCnt() (int, int, error) {
	ns, err := p.Cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{})
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}

	spotCnt := 0
	onDemandCnt := 0

	for i := range ns.Items {
		switch ns.Items[i].Annotations["eks.amazonaws.com/capacityType"] {
		case "ON_DEMAND":
			onDemandCnt++
		case "SPOT":
			spotCnt++
		}
	}
	return onDemandCnt, spotCnt, nil
}
