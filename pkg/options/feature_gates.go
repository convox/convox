package options

import (
	"os"
	"strings"
)

const (
	FeatureGateRdsDisable          = "rds-disable"
	FeatureGateElasticacheDisable  = "elasticache-disable"
	FeatureGateSystemEnvDisable    = "system-env-disable"
	FeatureGateBalancerDisable     = "balancer-disable"
	FeatureGateTid                 = "tid"
	FeatureGateAppLimitRequired    = "app-limit-required"
	FeatureGateExternalDnsResolver = "external-dns-resolver" // will use 1.1.1.1 as the default resolver if enabled
)

func GetFeatureGates() map[string]bool {
	featureGates := make(map[string]bool)
	featureGateStr := os.Getenv("FEATURE_GATES")
	for _, fg := range strings.Split(featureGateStr, ",") {
		parts := strings.SplitN(fg, "=", 2)
		if len(parts) != 2 {
			continue
		}
		featureGates[parts[0]] = parts[1] == "true"
	}
	return featureGates
}
