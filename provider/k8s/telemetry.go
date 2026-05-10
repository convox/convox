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

// rackUIDByNamespace caches namespace UID lookups keyed by namespace name.
// Kubernetes UIDs are immutable once set. Using sync.Map per namespace (not a
// single global string) means multiple Providers in the same process (unit
// tests, multi-rack CLI) get independent cached values.
var rackUIDByNamespace sync.Map

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
	// events (hash changes), and per-user uniqueness without leaking the
	// plaintext. Maintain ALPHABETICAL ORDER for ease of review when adding
	// new entries.
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

	// Build the merged param map. The redacted-params Secret (if present)
	// carries the real plaintext for keys whose ConfigMap entry has been
	// stubbed to empty string. Overlay Secret values over ConfigMap values
	// BEFORE redaction so the SHA-256 hashing operates on the real
	// credential. Older racks have no Secret; the Get returns NotFound
	// and we proceed with the ConfigMap values as before (graceful
	// backward compat).
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

// hashParamValue hashes the given plaintext credential value using HMAC-SHA256
// keyed by the rack namespace UID. This ensures that two racks hashing the same
// credential produce DIFFERENT ciphertext, preventing cross-rack correlation of
// credential rotation events in the telemetry receiver.
//
// Fallback: if the namespace UID lookup fails (e.g. during tests with a fake
// clientset that has no real UID), the HMAC key is derived from the rack name
// with a stable prefix so the result remains deterministic-per-rack without
// leaking the plaintext.
func (p *Provider) hashParamValue(value string) string {
	// Fast path: cached UID for this namespace.
	if cached, ok := rackUIDByNamespace.Load(p.Namespace); ok {
		if uid, ok2 := cached.(string); ok2 {
			mac := hmac.New(sha256.New, []byte(uid))
			mac.Write([]byte(value))
			return hex.EncodeToString(mac.Sum(nil))
		}
		// Cache poisoned with non-string entry; fall through to recompute.
	}

	// Slow path: fetch namespace to obtain its UID.
	uid := "convox-telemetry-v1:" + p.Namespace // fallback
	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), p.Namespace, am.GetOptions{})
	if err == nil && ns != nil && string(ns.UID) != "" {
		uid = string(ns.UID)
	}
	// Store under namespace name so different Providers (different namespaces)
	// get independent cached values. LoadOrStore is a no-op if another goroutine
	// raced and stored first; we still use the just-computed uid for this call.
	rackUIDByNamespace.LoadOrStore(p.Namespace, uid)

	mac := hmac.New(sha256.New, []byte(uid))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
