package k8s

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var rackUIDByNamespace sync.Map

var (
	skipParams = strings.Join([]string{
		"name",
		"rack_name",
		"release",
		"region",
	}, ",")

	// redactedParams are HMAC-hashed before emission to avoid leaking credentials.
	redactedParams = strings.Join([]string{
		"cidr",
		"docker_hub_password",
		"internet_gateway_id",
		"key_pair_name",
		"private_eks_pass",
		"prometheus_url",
		"syslog",
		"tags",
		"vpc_id",
		"webhook_signing_key",
		"whitelist",
	}, ",")
)

func (p *Provider) RackParams() map[string]interface{} {
	trp, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-rack-params", am.GetOptions{})
	if err != nil {
		fmt.Printf("could not find rack params configmap: %v", err)
		return nil
	}

	defaultParamValue := map[string]string{}
	trd, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-default-rack-params", am.GetOptions{})
	if err != nil {
		fmt.Printf("could not find rack default params configmap: %v", err)
	} else {
		defaultParamValue = trd.Data
	}

	// Overlay Secret values over ConfigMap before redaction (older racks have no Secret).
	merged := map[string]string{}
	for k, v := range trp.Data {
		merged[k] = v
	}
	tsec, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get(context.TODO(), "telemetry-rack-params-redacted", am.GetOptions{})
	if err == nil && tsec != nil {
		for k, v := range tsec.Data {
			merged[k] = string(v)
		}
	}

	toSync := map[string]interface{}{}
	for k, v := range merged {
		if strings.Contains(skipParams, k) {
			continue
		}

		if v == defaultParamValue[k] {
			continue
		}

		if strings.Contains(redactedParams, k) {
			v = p.hashParamValue(v)
		}

		toSync[k] = v
	}

	return toSync
}

// hashParamValue returns HMAC-SHA256 keyed by namespace UID so identical
// credentials on different racks produce different hashes.
func (p *Provider) hashParamValue(value string) string {
	if cached, ok := rackUIDByNamespace.Load(p.Namespace); ok {
		if uid, ok2 := cached.(string); ok2 {
			mac := hmac.New(sha256.New, []byte(uid))
			mac.Write([]byte(value))
			return hex.EncodeToString(mac.Sum(nil))
		}
	}

	uid := "convox-telemetry-v1:" + p.Namespace // fallback
	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), p.Namespace, am.GetOptions{})
	if err == nil && ns != nil && string(ns.UID) != "" {
		uid = string(ns.UID)
	}
	// Cache per-namespace; LoadOrStore handles concurrent callers.
	rackUIDByNamespace.LoadOrStore(p.Namespace, uid)

	mac := hmac.New(sha256.New, []byte(uid))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
