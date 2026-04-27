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

	// redactedParams: rack params whose VALUE is a credential or otherwise
	// sensitive, but whose PRESENCE is informative for telemetry. Values are
	// SHA-256-hashed before emission to metrics.convox.com — the receiver sees
	// an opaque hex string per param, signaling set-vs-unset, key-rotation
	// events (hash changes), and per-customer uniqueness without leaking the
	// plaintext. Maintain ALPHABETICAL ORDER for ease of review when adding
	// new entries.
	redactedParams = strings.Join([]string{
		"cidr",
		"docker_hub_password",
		"internet_gateway_id",
		"key_pair_name",
		"private_eks_pass",
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

	// Build the merged param map. Decision 8 — the redacted-params
	// Secret (if present) carries the real plaintext for keys whose
	// ConfigMap entry has been stubbed to empty string. Overlay
	// Secret values over ConfigMap values BEFORE redaction so the
	// SHA-256 hashing operates on the real credential. Pre-D8 racks
	// have no Secret; the Get returns NotFound and we proceed with
	// the ConfigMap values as before (graceful backward compat).
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
