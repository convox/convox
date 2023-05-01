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
		if !strings.Contains(skipParams, k) {
			if strings.Contains(redactedParams, k) {
				toSync[k] = hashParamValue(v)
			} else {
				toSync[k] = v
			}
		}
	}

	return toSync
}

func (p *Provider) CheckParamsInConfigMapAsSync() error {
	trs, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-rack-sync", am.GetOptions{})
	if err != nil {
		return err
	}

	for k, v := range trs.Data {
		if v == "false" {
			trs.Data[k] = "true"
		}
	}

	_, err = p.Cluster.CoreV1().ConfigMaps(p.Namespace).Update(context.TODO(), trs, am.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func hashParamValue(value string) string {
	hasher := sha256.New()
	hasher.Write([]byte(value))
	return hex.EncodeToString(hasher.Sum(nil))
}
