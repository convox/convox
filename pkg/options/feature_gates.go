package options

import (
	"os"
	"strings"
)

const (
	FeatureGateRdsDisable                   = "rds-disable"
	FeatureGateElasticacheDisable           = "elasticache-disable"
	FeatureGateSystemEnvDisable             = "system-env-disable"
	FeatureGateBalancerDisable              = "balancer-disable"
	FeatureGateTid                          = "tid"
	FeatureGateAppLimitRequired             = "app-limit-required"
	FeatureGateExternalDnsResolver          = "external-dns-resolver"           // will use 1.1.1.1 as the default resolver if enabled
	FeatureGateResourceInternalDomainSuffix = "resource-internal-domain-suffix" // will use svc.cluster.local as the default internal resource domain suffix
)

func GetFeatureGates() map[string]bool {
	featureGates := make(map[string]bool)
	featureGateStr := os.Getenv("FEATURE_GATES")
	for _, fg := range strings.Split(featureGateStr, ",") {
		parts := strings.SplitN(fg, "=", 2)
		if len(parts) != 2 {
			continue
		}
		featureGates[parts[0]] = parts[1] != ""
	}
	return featureGates
}

func GetFeatureGateValue(name string) string {
	featureGateStr := os.Getenv("FEATURE_GATES")
	for _, fg := range strings.Split(featureGateStr, ",") {
		parts := strings.SplitN(fg, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == name {
			return parts[1]
		}
	}
	return ""
}
