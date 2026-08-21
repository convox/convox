package options

import (
	"os"
	"strings"
)

const (
	FeatureGateRdsDisable                    = "rds-disable"
	FeatureGateElasticacheDisable            = "elasticache-disable"
	FeatureGateSystemEnvDisable              = "system-env-disable"
	FeatureGateBalancerDisable               = "balancer-disable"
	FeatureGateTid                           = "tid"
	FeatureGateAppLimitRequired              = "app-limit-required"
	FeatureGateExternalDnsResolver           = "external-dns-resolver"             // will use 1.1.1.1 as the default resolver if enabled
	FeatureGateResourceInternalDomainSuffix  = "resource-internal-domain-suffix"   // will use svc.cluster.local as the default internal resource domain suffix
	FeatureGateDisableHostUsersAsDefault     = "disable-host-users"                // will disable setting the host user in the container if enabled
	FeatureGatePrefixBasedAwsResourceDisable = "aws-resource-prefix-based-disable" // will reject resources named with the rds- and elasticache- prefixes
	FeatureGateRDSTemplateConfig             = "rds-template-config"               // names a config map that limits rds options to a curated set
	FeatureGateDeployFastFailDisable         = "deploy-fast-fail-disable"          // turns off deploy fast-fail detection rack-wide
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
