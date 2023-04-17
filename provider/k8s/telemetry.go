package k8s

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	skipParams = []string{
		"name",
		"rack_name",
		"cidr",
		"key_pair_name",
		"internet_gateway_id",
		"syslog",
		"tags",
		"vpc_id",
		"whitelist",
	}
	redactedParams = strings.Join(skipParams, ",")
)

func (p *Provider) RackParams() map[string]interface{} {
	trp, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-rack-params", am.GetOptions{})
	if err != nil {
		fmt.Printf("could not find rack params configmap: %v", err)
		return nil
	}

	trs, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-rack-sync", am.GetOptions{})
	if err != nil {
		fmt.Printf("could not find params sync configmap: %v: Creating configmap...", err)

		err = createNewSyncConfigMap(trp, trs, p)
		if err != nil {
			return map[string]interface{}{}
		}
	}

	// Get all new params and update trs configmap
	newParams := []string{}
	for k := range trp.Data {
		if _, ok := trs.Data[k]; !ok {
			newParams = append(newParams, k)
		}
	}

	if len(newParams) > 0 {
		for _, np := range newParams {
			trs.Data[np] = "false"
		}

		trs, err = p.Cluster.CoreV1().ConfigMaps(p.Namespace).Update(context.TODO(), trs, am.UpdateOptions{})
		if err != nil {
			return nil
		}
	}

	// Get all non sync params
	var nSync []string
	for k, v := range trs.Data {
		if v == "false" {
			nSync = append(nSync, k)
		}
	}

	// Get all non sync from initial configmap and return them
	toSync := map[string]interface{}{}
	for _, s := range nSync {
		if val, ok := trp.Data[s]; ok {
			if strings.Contains(redactedParams, s) {
				toSync[s] = hashParamValue(val)
			} else {
				toSync[s] = val
			}
		}
	}

	return toSync
}

func createNewSyncConfigMap(trp *v1.ConfigMap, trs *v1.ConfigMap, p *Provider) error {
	data := map[string]string{}
	for k := range trp.Data {
		data[k] = "false"
	}

	cm := &v1.ConfigMap{
		ObjectMeta: am.ObjectMeta{
			Name: "telemetry-rack-sync",
		},
		Data: data,
	}

	trs, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
	if err != nil {
		fmt.Printf("could not create configmap: %v", err)
		return err
	}

	fmt.Printf("Created Sync ConfigMap %s\n", trs.GetName())
	return nil
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
	hasher := sha1.New()
	hasher.Write([]byte(value))
	return hex.EncodeToString(hasher.Sum(nil))
}
