package k8s

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	skipParams = strings.Join([]string{
		"name",
		"rack_name",
		"release",
		"region",
	}, ",")

	redactedParams = strings.Join([]string{
		"cidr",
		"key_pair_name",
		"internet_gateway_id",
		"syslog",
		"tags",
		"vpc_id",
		"whitelist",
	}, ",")
)

func (p *Provider) RackParams() map[string]interface{} {
	trp, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-rack-params", am.GetOptions{})
	if err != nil {
		fmt.Printf("could not find rack params configmap: %v", err)
		return nil
	}

	toSync := map[string]interface{}{}
	for k, v := range trp.Data {
		if strings.Contains(skipParams, k) {
			continue
		}

		if strings.Contains(redactedParams, k) {
			v = hashParamValue(v)
		}

		toSync[k] = v
	}

	return toSync
}

func hashParamValue(value string) string {
	hasher := sha256.New()
	hasher.Write([]byte(value))
	return hex.EncodeToString(hasher.Sum(nil))
}
