package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

var providerKnownParams = map[string]map[string]bool{
	"aws":   awsKnownParams,
	"gcp":   gcpKnownParams,
	"azure": azureKnownParams,
	"do":    doKnownParams,
	"metal": metalKnownParams,
	"local": localKnownParams,
}

var awsKnownParams = map[string]bool{
	"access_log_retention_in_days": true, "additional_build_groups_config": true,
	"additional_karpenter_nodepools_config": true, "additional_node_groups_config": true,
	"api_feature_gates": true, "availability_zones": true,
	"aws_ebs_csi_driver_version": true, "build_disable_convox_resolver": true,
	"build_node_enabled": true, "build_node_min_count": true,
	"build_node_type": true, "buildkit_host_path_cache_enable": true,
	"cert_duration": true, "cidr": true,
	"convox_domain_tls_cert_disable": true, "convox_rack_domain": true,
	"coredns_version": true, "cost_tracking_enable": true, "custom_provided_bucket": true,
	"deploy_extra_nlb": true, "disable_convox_resolver": true,
	"disable_image_manifest_cache": true, "disable_public_access": true,
	"docker_hub_password": true, "docker_hub_username": true,
	"ebs_volume_encryption_enabled": true, "ecr_docker_hub_cache": true, "ecr_scan_on_push_enable": true,
	"efs_csi_driver_enable": true, "efs_csi_driver_version": true,
	"eks_api_server_public_access_cidrs": true, "enable_private_access": true,
	"fluentd_disable": true, "fluentd_memory": true,
	"gpu_tag_enable": true, "high_availability": true,
	"idle_timeout": true, "image": true,
	"imds_http_hop_limit": true, "imds_http_tokens": true,
	"imds_tags_enable": true, "internal_router": true,
	"internet_gateway_id": true, "k8s_version": true,
	"karpenter_arch": true, "karpenter_auth_mode": true,
	"karpenter_build_capacity_types": true, "karpenter_build_consolidate_after": true,
	"karpenter_build_cpu_limit": true, "karpenter_build_instance_families": true,
	"karpenter_build_instance_sizes": true, "karpenter_build_memory_limit_gb": true,
	"karpenter_build_node_labels": true, "karpenter_capacity_types": true,
	"karpenter_config": true, "karpenter_consolidate_after": true,
	"karpenter_consolidation_enabled": true, "karpenter_cpu_limit": true,
	"karpenter_disruption_budget_nodes": true, "karpenter_enabled": true,
	"karpenter_instance_families": true, "karpenter_instance_sizes": true,
	"karpenter_memory_limit_gb": true, "karpenter_node_disk": true,
	"karpenter_node_expiry": true, "karpenter_node_labels": true,
	"karpenter_node_taints": true, "karpenter_node_volume_type": true,
	"keda_enable": true, "key_pair_name": true,
	"kube_proxy_version": true, "kubelet_registry_burst": true,
	"kubelet_registry_pull_qps": true, "max_on_demand_count": true,
	"min_on_demand_count": true, "name": true,
	"nginx_additional_config": true, "nginx_image": true,
	"nlb_security_group": true, "node_capacity_type": true,
	"node_disk": true, "node_max_unavailable_percentage": true,
	"node_type": true, "nvidia_device_plugin_enable": true,
	"nvidia_device_time_slicing_replicas": true, "pdb_default_min_available_percentage": true,
	"pod_identity_agent_enable": true, "pod_identity_agent_version": true,
	"private": true, "private_eks_host": true,
	"private_eks_pass": true, "private_eks_user": true,
	"private_subnets_ids": true, "proxy_protocol": true,
	"public_subnets_ids": true, "rack_name": true,
	"region": true, "release": true,
	"releases_to_retain_after_active": true, "releases_to_retain_task_run_interval_hour": true,
	"schedule_rack_scale_down": true, "schedule_rack_scale_up": true,
	"settings": true, "ssl_ciphers": true,
	"ssl_protocols": true, "sync_tf_now": true,
	"syslog": true, "tags": true,
	"telemetry": true, "terraform_update_timeout": true,
	"user_data": true, "user_data_url": true,
	"vpa_enable": true, "vpc_cni_version": true,
	"vpc_id": true, "whitelist": true,
}

var gcpKnownParams = map[string]bool{
	"buildkit_enabled": true, "cert_duration": true, "docker_hub_password": true,
	"docker_hub_username": true, "fluentd_memory": true, "image": true,
	"k8s_version": true, "name": true, "nginx_additional_config": true,
	"node_disk": true, "node_type": true, "preemptible": true,
	"rack_name": true, "region": true, "release": true, "settings": true,
	"sync_tf_now": true, "syslog": true, "telemetry": true,
	"terraform_update_timeout": true, "whitelist": true,
}

var azureKnownParams = map[string]bool{
	"additional_build_groups_config": true, "additional_node_groups_config": true,
	"azure_files_enable": true, "cert_duration": true,
	"docker_hub_password": true, "docker_hub_username": true,
	"fluentd_memory": true, "high_availability": true, "idle_timeout": true,
	"image": true, "k8s_version": true, "max_on_demand_count": true,
	"min_on_demand_count": true, "name": true, "nginx_additional_config": true,
	"nginx_image": true, "node_disk": true, "node_type": true,
	"nvidia_device_plugin_enable": true, "nvidia_device_time_slicing_replicas": true,
	"pdb_default_min_available_percentage": true, "rack_name": true, "region": true,
	"release": true, "settings": true, "ssl_ciphers": true, "ssl_protocols": true,
	"sync_tf_now": true, "syslog": true, "tags": true, "telemetry": true,
	"terraform_update_timeout": true, "whitelist": true,
}

var doKnownParams = map[string]bool{
	"access_id": true, "buildkit_enabled": true, "cert_duration": true,
	"docker_hub_password": true, "docker_hub_username": true, "fluentd_memory": true,
	"high_availability": true, "image": true, "k8s_version": true,
	"name": true, "node_type": true, "rack_name": true, "region": true,
	"registry_disk": true, "release": true, "secret_key": true,
	"settings": true, "sync_tf_now": true, "syslog": true, "telemetry": true,
	"terraform_update_timeout": true, "token": true, "whitelist": true,
}

var metalKnownParams = map[string]bool{
	"docker_hub_password": true, "docker_hub_username": true, "domain": true,
	"fluentd_memory": true, "image": true, "name": true, "rack_name": true,
	"registry_disk": true, "release": true, "sync_tf_now": true,
	"syslog": true, "whitelist": true,
}

var localKnownParams = map[string]bool{
	"docker_hub_password": true, "docker_hub_username": true, "image": true,
	"name": true, "os": true, "rack_name": true, "release": true,
	"settings": true, "sync_tf_now": true, "telemetry": true,
}

var managedParams = map[string]bool{
	"image": true, "name": true, "rack_name": true, "release": true, "settings": true,
	"nginx_image": true, "k8s_version": true, "aws_ebs_csi_driver_version": true,
	"coredns_version": true, "efs_csi_driver_version": true, "kube_proxy_version": true,
	"pod_identity_agent_version": true, "vpc_cni_version": true,
	"disable_public_access": true, "enable_private_access": true,
	"eks_api_server_public_access_cidrs": true,
}

// sensitiveParams enumerates rack params whose values are rendered as
// "**********" on a TTY when --reveal is not passed. Pipe output and
// --reveal bypass masking.
//
// Lineage: always-mask introduced in 3.24.4 (docker_hub_password,
// secret_key, token); list extended and TTY-gating + --reveal added
// alongside this var's package-level promotion in 3.24.5 (access_id,
// private_eks_host, private_eks_user, private_eks_pass). Password and
// HttpProxy are v2-rack PascalCase keys included so v3 CLI against a
// v2 rack masks the same values v2 CLI post-PR-3795 does; they never
// appear in v3 rack Parameters responses (v3 uses snake_case).
var sensitiveParams = map[string]bool{
	"docker_hub_password": true,
	"secret_key":          true,
	"token":               true,
	"access_id":           true,
	"private_eks_host":    true,
	"private_eks_user":    true,
	"private_eks_pass":    true,
	"Password":            true,
	"HttpProxy":           true,
}

// paramGroups categorizes rack params into curated logical groups for the
// `convox rack params -g <group>` filter. A param may belong to multiple
// groups. Params not listed here are shown in the default view but not
// surfaced by any group filter.
var paramGroups = map[string]map[string]bool{
	"karpenter": {
		"additional_karpenter_nodepools_config": true,
		"karpenter_arch":                        true,
		"karpenter_auth_mode":                   true,
		"karpenter_build_capacity_types":        true,
		"karpenter_build_consolidate_after":     true,
		"karpenter_build_cpu_limit":             true,
		"karpenter_build_instance_families":     true,
		"karpenter_build_instance_sizes":        true,
		"karpenter_build_memory_limit_gb":       true,
		"karpenter_build_node_labels":           true,
		"karpenter_capacity_types":              true,
		"karpenter_config":                      true,
		"karpenter_consolidate_after":           true,
		"karpenter_consolidation_enabled":       true,
		"karpenter_cpu_limit":                   true,
		"karpenter_disruption_budget_nodes":     true,
		"karpenter_enabled":                     true,
		"karpenter_instance_families":           true,
		"karpenter_instance_sizes":              true,
		"karpenter_memory_limit_gb":             true,
		"karpenter_node_disk":                   true,
		"karpenter_node_expiry":                 true,
		"karpenter_node_labels":                 true,
		"karpenter_node_taints":                 true,
		"karpenter_node_volume_type":            true,
	},
	"network": {
		// v3 native (snake_case)
		"availability_zones":      true,
		"cidr":                    true,
		"deploy_extra_nlb":        true,
		"disable_convox_resolver": true,
		"internal_router":         true,
		"internet_gateway_id":     true,
		"nlb_security_group":      true,
		"private_eks_host":        true,
		"private_subnets_ids":     true,
		"proxy_protocol":          true,
		"public_subnets_ids":      true,
		"vpc_id":                  true,
		// v2 PascalCase (no-op on v3 racks; surfaced on v2 racks)
		"AvailabilityZones":    true,
		"ExistingVpc":          true,
		"HttpProxy":            true, // dual-listed in security
		"Internal":             true,
		"InternalOnly":         true,
		"InternetGateway":      true,
		"MaxAvailabilityZones": true,
		"PlaceLambdaInVpc":     true,
		"Private":              true,
		"Subnet0CIDR":          true,
		"Subnet1CIDR":          true,
		"Subnet2CIDR":          true,
		"SubnetPrivate0CIDR":   true,
		"SubnetPrivate1CIDR":   true,
		"SubnetPrivate2CIDR":   true,
		"VPCCIDR":              true,
	},
	"security": {
		// v3 native (snake_case)
		"access_id":                          true,
		"disable_public_access":              true,
		"docker_hub_password":                true,
		"ebs_volume_encryption_enabled":      true,
		"ecr_scan_on_push_enable":            true,
		"eks_api_server_public_access_cidrs": true,
		"enable_private_access":              true,
		"imds_http_hop_limit":                true,
		"imds_http_tokens":                   true,
		"imds_tags_enable":                   true,
		"nlb_security_group":                 true,
		"pod_identity_agent_enable":          true,
		"private_eks_host":                   true,
		"private_eks_pass":                   true,
		"private_eks_user":                   true,
		"secret_key":                         true,
		"ssl_ciphers":                        true,
		"ssl_protocols":                      true,
		"token":                              true,
		"whitelist":                          true,
		// v2 PascalCase (no-op on v3 racks; surfaced on v2 racks)
		"BuildInstancePolicy":                   true, // dual-listed in build
		"BuildInstanceSecurityGroup":            true,
		"EnableContainerReadonlyRootFilesystem": true,
		"EnableSharedEFSVolumeEncryption":       true, // dual-listed in storage
		"EncryptEbs":                            true, // dual-listed in storage
		"Encryption":                            true,
		"HttpProxy":                             true, // dual-listed in network
		"IMDSHttpPutResponseHopLimit":           true,
		"IMDSHttpTokens":                        true,
		"InstancePolicy":                        true, // dual-listed in nodes
		"InstanceSecurityGroup":                 true,
		"InstancesIpToIncludInWhiteListing":     true,
		"Key":                                   true,
		"Password":                              true,
		"PrivateApiSecurityGroup":               true,
		"RouterInternalSecurityGroup":           true,
		"RouterSecurityGroup":                   true,
		"SslPolicy":                             true,
		"WhiteList":                             true,
	},
	"scaling": {
		// v3 native (snake_case)
		"high_availability":                    true,
		"karpenter_disruption_budget_nodes":    true,
		"keda_enable":                          true,
		"max_on_demand_count":                  true,
		"min_on_demand_count":                  true,
		"node_max_unavailable_percentage":      true,
		"pdb_default_min_available_percentage": true,
		"schedule_rack_scale_down":             true,
		"schedule_rack_scale_up":               true,
		"vpa_enable":                           true,
		// v2 PascalCase (no-op on v3 racks; surfaced on v2 racks)
		"Autoscale":                      true,
		"AutoscaleExtra":                 true,
		"HighAvailability":               true,
		"InstanceCount":                  true,
		"InstanceUpdateBatchSize":        true,
		"NoHAAutoscaleExtra":             true,
		"NoHaInstanceCount":              true,
		"OnDemandMinCount":               true,
		"ScheduleRackScaleDown":          true,
		"ScheduleRackScaleUp":            true,
		"SpotFleetAllocationStrategy":    true,
		"SpotFleetAllowedInstanceTypes":  true,
		"SpotFleetExcludedInstanceTypes": true,
		"SpotFleetMaxPrice":              true,
		"SpotFleetMinMemoryMiB":          true,
		"SpotFleetMinOnDemandCount":      true,
		"SpotFleetMinVcpuCount":          true,
		"SpotFleetTargetType":            true,
		"SpotInstanceBid":                true,
	},
	"nodes": {
		// v3 native (snake_case)
		"additional_node_groups_config":       true,
		"gpu_tag_enable":                      true,
		"key_pair_name":                       true,
		"kubelet_registry_burst":              true,
		"kubelet_registry_pull_qps":           true,
		"node_capacity_type":                  true,
		"node_disk":                           true,
		"node_max_unavailable_percentage":     true,
		"node_type":                           true,
		"nvidia_device_plugin_enable":         true,
		"nvidia_device_time_slicing_replicas": true,
		"os":                                  true,
		"preemptible":                         true,
		"user_data":                           true,
		"user_data_url":                       true,
		// v2 PascalCase (no-op on v3 racks; v2 "instances" group content)
		"Ami":                 true,
		"CpuCredits":          true,
		"DefaultAmi":          true,
		"DefaultAmiArm":       true,
		"InstanceBootCommand": true,
		"InstancePolicy":      true, // dual-listed in security
		"InstanceRunCommand":  true,
		"InstanceType":        true,
		"SwapSize":            true,
		"Tenancy":             true,
		"VolumeSize":          true,
	},
	"build": {
		// v3 native (snake_case)
		"additional_build_groups_config":    true,
		"build_disable_convox_resolver":     true,
		"build_node_enabled":                true,
		"build_node_min_count":              true,
		"build_node_type":                   true,
		"buildkit_enabled":                  true,
		"buildkit_host_path_cache_enable":   true,
		"karpenter_build_capacity_types":    true,
		"karpenter_build_consolidate_after": true,
		"karpenter_build_cpu_limit":         true,
		"karpenter_build_instance_families": true,
		"karpenter_build_instance_sizes":    true,
		"karpenter_build_memory_limit_gb":   true,
		"karpenter_build_node_labels":       true,
		// v2 PascalCase (no-op on v3 racks; surfaced on v2 racks)
		"BuildCpu":                    true,
		"BuildImage":                  true,
		"BuildInstance":               true,
		"BuildInstancePolicy":         true, // dual-listed in security
		"BuildMemory":                 true,
		"BuildMethod":                 true,
		"BuildVolumeSize":             true,
		"FargateBuildCpu":             true,
		"FargateBuildMemory":          true,
		"PrivateBuild":                true,
		"PruneOlderImagesCronRunFreq": true,
		"PruneOlderImagesInHour":      true,
	},
	// docker_hub_password is dual-listed in "security" (above) because it is
	// a masked credential; docker_hub_username stays registry-only as a
	// public identifier (matches existing non-masked convention).
	"registry": {
		"custom_provided_bucket":       true,
		"disable_image_manifest_cache": true,
		"docker_hub_password":          true,
		"docker_hub_username":          true,
		"ecr_docker_hub_cache":         true,
		"ecr_scan_on_push_enable":      true,
	},
	"logging": {
		// v3 native (snake_case)
		"access_log_retention_in_days": true,
		"fluentd_disable":              true,
		"fluentd_memory":               true,
		"syslog":                       true,
		"telemetry":                    true,
		// v2 PascalCase (no-op on v3 racks; surfaced on v2 racks)
		"LogBucket":         true,
		"LogDriver":         true,
		"LogRetention":      true,
		"SyslogDestination": true,
		"SyslogFormat":      true,
	},
	"ingress": {
		"cert_duration":           true,
		"idle_timeout":            true,
		"nginx_additional_config": true,
		"nginx_image":             true,
		"ssl_ciphers":             true,
		"ssl_protocols":           true,
	},
	"domain": {
		"convox_domain_tls_cert_disable": true,
		"convox_rack_domain":             true,
		"domain":                         true,
	},
	"storage": {
		// v3 native (snake_case)
		"aws_ebs_csi_driver_version":    true,
		"azure_files_enable":            true,
		"ebs_volume_encryption_enabled": true,
		"efs_csi_driver_enable":         true,
		"efs_csi_driver_version":        true,
		"registry_disk":                 true,
		// v2 PascalCase (no-op on v3 racks; surfaced on v2 racks)
		"DynamoDbTableDeletionProtectionEnabled":  true,
		"DynamoDbTablePointInTimeRecoveryEnabled": true,
		"EnableS3Versioning":                      true,
		"EnableSharedEFSVolumeEncryption":         true, // dual-listed in security
		"EncryptEbs":                              true, // dual-listed in security
	},
	"retention": {
		"releases_to_retain_after_active":           true,
		"releases_to_retain_task_run_interval_hour": true,
	},
	"versions": {
		"aws_ebs_csi_driver_version": true,
		"coredns_version":            true,
		"efs_csi_driver_version":     true,
		"k8s_version":                true,
		"kube_proxy_version":         true,
		"nginx_image":                true,
		"pod_identity_agent_version": true,
		"vpc_cni_version":            true,
	},
	// nlb is v2-only content (v3 has no CloudFormation NLB config params; v3's
	// NLB-adjacent keys — nlb_security_group, deploy_extra_nlb — live in the
	// network group above). The group exists so v2 rack users running `convox
	// rack params -g nlb` through v3 CLI get the same filter surface they have
	// in v2 CLI. On a v3 rack the groupFilter matches zero keys and the
	// "no params in group 'nlb' for this rack" NOTICE fires.
	"nlb": {
		"NLB":                           true,
		"NLBAllowCIDR":                  true,
		"NLBCrossZone":                  true,
		"NLBDeletionProtection":         true,
		"NLBInternal":                   true,
		"NLBInternalAllowCIDR":          true,
		"NLBInternalCrossZone":          true,
		"NLBInternalDeletionProtection": true,
		"NLBInternalPreserveClientIP":   true,
		"NLBPreserveClientIP":           true,
	},
}

// groupDescriptions provides the one-line label shown next to each group
// name in error output (e.g., unknown-group and ambiguous-prefix errors).
// Keep in sync with paramGroups keys.
var groupDescriptions = map[string]string{
	"karpenter": "Karpenter autoscaling configuration",
	"network":   "VPC, subnets, CIDR, routing, NLB, DNS resolver",
	"nlb":       "NLB config: listeners, cross-zone, allow-CIDR, preserve-client-IP, deletion protection (v2 racks)",
	"security":  "access controls, whitelist, IAM, encryption, private EKS, IMDS, TLS, credentials",
	"scaling":   "capacity counts, HA, HPA/VPA/KEDA, schedules, PDB, disruption budgets",
	"nodes":     "default node-group config, user-data, GPU, kubelet tuning",
	"build":     "build node config, buildkit, additional build groups",
	"registry":  "Docker Hub, ECR, image caching, storage buckets",
	"logging":   "syslog, telemetry, fluentd",
	"ingress":   "NGINX, idle timeout, TLS cert duration",
	"domain":    "rack domain and TLS toggle",
	"storage":   "CSI drivers, EBS/EFS/Azure Files, registry disk",
	"retention": "release retention policy",
	"versions":  "K8s and managed component versions",
}

// resolveGroup resolves a possibly-partial group name to an exact group key.
// Priority: exact match > unique prefix match. Case-insensitive. Whitespace
// is trimmed. Returns an error listing candidates or all groups on
// ambiguous / unknown input.
func resolveGroup(input string) (string, error) {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return "", fmt.Errorf("group name required\n  %s", formatGroupList())
	}

	if _, ok := paramGroups[input]; ok {
		return input, nil
	}

	var matches []string
	for g := range paramGroups {
		if strings.HasPrefix(g, input) {
			matches = append(matches, g)
		}
	}
	sort.Strings(matches)

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("group '%s' not found\n  %s", input, formatGroupList())
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("group '%s' matches multiple groups: %s %s\n  %s",
			input, strings.Join(matches, ", "), formatAmbiguousHint(matches), formatGroupList())
	}
}

// formatGroupList returns a sorted, padded two-column listing of all
// available groups for inclusion in error output.
func formatGroupList() string {
	names := make([]string, 0, len(groupDescriptions))
	maxLen := 0
	for g := range groupDescriptions {
		names = append(names, g)
		if len(g) > maxLen {
			maxLen = len(g)
		}
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("available groups:\n")
	for _, g := range names {
		b.WriteString(fmt.Sprintf("  %-*s    %s\n", maxLen, g, groupDescriptions[g]))
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatAmbiguousHint returns a parenthesized hint showing a short-but-
// readable disambiguating prefix for each ambiguous candidate, e.g.,
// "(use 'net' or 'nod')" for candidates ["network", "nodes"].
func formatAmbiguousHint(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	hints := make([]string, 0, len(candidates))
	for _, c := range candidates {
		hints = append(hints, "'"+disambiguatingPrefix(c)+"'")
	}
	switch len(hints) {
	case 1:
		return "(use " + hints[0] + ")"
	case 2:
		return "(use " + hints[0] + " or " + hints[1] + ")"
	default:
		return "(use " + strings.Join(hints[:len(hints)-1], ", ") + ", or " + hints[len(hints)-1] + ")"
	}
}

// disambiguatingPrefix returns a short-but-readable prefix of `group` that
// resolves uniquely against all paramGroups keys. Uses a 3-character
// minimum for human readability — a technically-unique 1- or 2-char
// prefix like "no" for "nodes" reads like negation and is avoided.
func disambiguatingPrefix(group string) string {
	const minLen = 3
	if len(group) <= minLen {
		return group
	}
	for n := minLen; n <= len(group); n++ {
		prefix := group[:n]
		hits := 0
		for g := range paramGroups {
			if strings.HasPrefix(g, prefix) {
				hits++
				if hits > 1 {
					break
				}
			}
		}
		if hits == 1 {
			return prefix
		}
	}
	return group
}

func init() {
	register("rack", "get information about the rack", watch(Rack), stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagWatchInterval},
		Validate: stdcli.Args(0),
	})

	register("rack access", "get rack access credential", RackAccess, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.StringFlag("role", "", "access role: read, write, or admin"),
			stdcli.IntFlag("duration-in-hours", "", "duration in hours"),
		},
		Validate: stdcli.Args(0),
	})

	register("rack access key rotate", "rotate access key", RackAccessKeyRotate, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack install", "install a new rack", RackInstall, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			stdcli.BoolFlag("prepare", "", "prepare the install but don't run it"),
			stdcli.StringFlag("version", "v", "rack version"),
			stdcli.StringFlag("runtime", "r", "runtime id"),
		},
		Usage:    "<provider> <name> [option=value]...",
		Validate: stdcli.ArgsMin(2),
	})

	register("rack karpenter cleanup", "clean up orphaned Karpenter nodes after disabling Karpenter", RackKarpenterCleanup, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "",
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack kubeconfig", "generate kubeconfig for rack", RackKubeconfig, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	register("rack logs", "get logs for the rack", RackLogs, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.LogsOptions{}), flagNoFollow, flagRack),
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack mv", "move a rack to or from console", RackMv, stdcli.CommandOptions{
		Usage:    "<from> <to>",
		Validate: stdcli.Args(2),
	})

	registerWithoutProvider("rack params", "display rack parameters", RackParams, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.StringFlag("group", "g", "filter to a param group (invalid name lists all)"),
			stdcli.BoolFlag("reveal", "", "show unmasked param values"),
		},
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack params set", "set rack parameters", RackParamsSet, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagForceParams},
		Usage:    "<Key=Value> [Key=Value]...",
		Validate: stdcli.ArgsMin(1),
	})

	register("rack ps", "list rack processes", RackPs, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.SystemProcessesOptions{}), flagRack),
		Validate: stdcli.Args(0),
	})

	register("rack releases", "list rack version history", RackReleases, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	register("rack runtimes", "list attachable runtime integrations", RackRuntimes, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	register("rack runtime attach", "attach runtime integration", RackRuntimeAttach, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(1),
	})

	register("rack scale", "scale the rack", RackScale, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.IntFlag("count", "c", "instance count"),
			stdcli.StringFlag("type", "t", "instance type"),
		},
		Validate: stdcli.Args(0),
	})

	register("rack sync", "sync v2 rack API url", RackSync, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack uninstall", "uninstall a rack", RackUninstall, stdcli.CommandOptions{
		Usage:    "<name>",
		Validate: stdcli.Args(1),
	})

	registerWithoutProvider("rack update", "update a rack", RackUpdate, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagForce},
		Usage:    "[version]",
		Validate: stdcli.ArgsMax(1),
	})
}

type NodeGroupConfigParam struct {
	Id           *int    `json:"id"`
	Type         string  `json:"type"`
	Disk         *int    `json:"disk,omitempty"`
	CapacityType *string `json:"capacity_type,omitempty"`
	MinSize      *int    `json:"min_size,omitempty"`
	MaxSize      *int    `json:"max_size,omitempty"`
	Label        *string `json:"label,omitempty"`
	AmiID        *string `json:"ami_id,omitempty"`
	Dedicated    *bool   `json:"dedicated,omitempty"`
	Tags         *string `json:"tags,omitempty"`
}

func (n *NodeGroupConfigParam) Validate() error {
	if n.Type == "" {
		return fmt.Errorf("node type is required: '%s'", n.Type)
	}
	if n.Disk != nil && *n.Disk < 20 {
		return fmt.Errorf("node disk is less than 20: '%d'", *n.Disk)
	}
	if n.MinSize != nil && *n.MinSize < 0 {
		return fmt.Errorf("invalid min size: '%d'", *n.MinSize)
	}
	if n.MaxSize != nil && *n.MaxSize < 0 {
		return fmt.Errorf("invalid max size: '%d'", *n.MaxSize)
	}
	if n.MinSize != nil && n.MaxSize != nil && *n.MinSize > *n.MaxSize {
		return fmt.Errorf("invalid min size: '%d' must be less or equal to max size", *n.MinSize)
	}
	if n.CapacityType != nil && (*n.CapacityType != "ON_DEMAND" && *n.CapacityType != "SPOT" && *n.CapacityType != "Regular" && *n.CapacityType != "Spot") {
		return fmt.Errorf("allowed capacity type: ON_DEMAND, SPOT, Regular, or Spot, found: '%s'", *n.CapacityType)
	}
	if n.Label != nil && !manifest.NameValidator.MatchString(*n.Label) {
		return fmt.Errorf("label value '%s' invalid, %s", *n.Label, manifest.ValidNameDescription)
	}

	if n.Dedicated != nil && *n.Dedicated && n.Label == nil {
		return fmt.Errorf("label is required when dedicated option is set")
	}

	if n.Tags != nil {
		reserved := []string{"name", "rack"}
		for _, part := range strings.Split(*n.Tags, ",") {
			if len(strings.SplitN(part, "=", 2)) != 2 {
				return fmt.Errorf("invalid 'tags', use format: k1=v1,k2=v2")
			}

			k := strings.SplitN(part, "=", 2)[0]
			if common.ContainsInStringSlice(reserved, strings.ToLower(k)) {
				return fmt.Errorf("reserved tag key '%s' is not allowed", k)
			}
		}
	}

	return nil
}

type KarpenterNodePoolConfigParam struct {
	Name                  string  `json:"name"`
	InstanceFamilies      *string `json:"instance_families,omitempty"`
	InstanceSizes         *string `json:"instance_sizes,omitempty"`
	CapacityTypes         *string `json:"capacity_types,omitempty"`
	Arch                  *string `json:"arch,omitempty"`
	CpuLimit              *int    `json:"cpu_limit,omitempty"`
	MemoryLimitGb         *int    `json:"memory_limit_gb,omitempty"`
	ConsolidationPolicy   *string `json:"consolidation_policy,omitempty"`
	ConsolidateAfter      *string `json:"consolidate_after,omitempty"`
	NodeExpiry            *string `json:"node_expiry,omitempty"`
	DisruptionBudgetNodes *string `json:"disruption_budget_nodes,omitempty"`
	Disk                  *int    `json:"disk,omitempty"`
	VolumeType            *string `json:"volume_type,omitempty"`
	Labels                *string `json:"labels,omitempty"`
	Taints                *string `json:"taints,omitempty"`
	Dedicated             *bool   `json:"dedicated,omitempty"`
	Weight                *int    `json:"weight,omitempty"`
}

var karpenterNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

func (np *KarpenterNodePoolConfigParam) Validate() error {
	if np.Name == "" {
		return fmt.Errorf("karpenter nodepool name is required")
	}
	if !karpenterNameRe.MatchString(np.Name) {
		return fmt.Errorf("karpenter nodepool name '%s' must be lowercase alphanumeric with dashes, max 63 chars", np.Name)
	}
	reserved := map[string]bool{"workload": true, "build": true, "default": true, "system": true}
	if reserved[np.Name] {
		return fmt.Errorf("karpenter nodepool name '%s' is reserved", np.Name)
	}

	if np.CapacityTypes != nil {
		for _, ct := range strings.Split(*np.CapacityTypes, ",") {
			ct = strings.TrimSpace(ct)
			if ct != "on-demand" && ct != "spot" {
				return fmt.Errorf("karpenter nodepool '%s': invalid capacity type '%s' (must be on-demand or spot)", np.Name, ct)
			}
		}
	}

	if np.Arch != nil {
		for _, a := range strings.Split(*np.Arch, ",") {
			a = strings.TrimSpace(a)
			if a != "amd64" && a != "arm64" {
				return fmt.Errorf("karpenter nodepool '%s': invalid arch '%s' (must be amd64 or arm64)", np.Name, a)
			}
		}
	}

	if np.CpuLimit != nil && *np.CpuLimit <= 0 {
		return fmt.Errorf("karpenter nodepool '%s': cpu_limit must be positive", np.Name)
	}
	if np.MemoryLimitGb != nil && *np.MemoryLimitGb <= 0 {
		return fmt.Errorf("karpenter nodepool '%s': memory_limit_gb must be positive", np.Name)
	}

	durationRe := regexp.MustCompile(`^\d+[smh]$`)
	if np.ConsolidateAfter != nil && !durationRe.MatchString(*np.ConsolidateAfter) {
		return fmt.Errorf("karpenter nodepool '%s': consolidate_after must be a duration like 30s, 5m, or 1h", np.Name)
	}

	if np.NodeExpiry != nil {
		expiryRe := regexp.MustCompile(`^\d+h$`)
		if *np.NodeExpiry != "Never" && !expiryRe.MatchString(*np.NodeExpiry) {
			return fmt.Errorf("karpenter nodepool '%s': node_expiry must be a duration like 720h or Never", np.Name)
		}
	}

	if np.DisruptionBudgetNodes != nil {
		budgetRe := regexp.MustCompile(`^\d+%?$`)
		if !budgetRe.MatchString(*np.DisruptionBudgetNodes) {
			return fmt.Errorf("karpenter nodepool '%s': disruption_budget_nodes must be a number or percentage", np.Name)
		}
	}

	if np.ConsolidationPolicy != nil {
		if *np.ConsolidationPolicy != "WhenEmpty" && *np.ConsolidationPolicy != "WhenEmptyOrUnderutilized" {
			return fmt.Errorf("karpenter nodepool '%s': consolidation_policy must be WhenEmpty or WhenEmptyOrUnderutilized", np.Name)
		}
	}

	if np.VolumeType != nil {
		validVols := map[string]bool{"gp2": true, "gp3": true, "io1": true, "io2": true}
		if !validVols[*np.VolumeType] {
			return fmt.Errorf("karpenter nodepool '%s': volume_type must be gp2, gp3, io1, or io2", np.Name)
		}
	}

	if np.Disk != nil && *np.Disk < 0 {
		return fmt.Errorf("karpenter nodepool '%s': disk must be non-negative", np.Name)
	}

	if np.Weight != nil && (*np.Weight < 0 || *np.Weight > 100) {
		return fmt.Errorf("karpenter nodepool '%s': weight must be 0-100", np.Name)
	}

	if np.Labels != nil && *np.Labels != "" {
		if strings.Contains(*np.Labels, `"`) {
			return fmt.Errorf("karpenter nodepool '%s': label keys and values must not contain double quotes", np.Name)
		}
		reservedPoolLabels := map[string]bool{"convox.io/nodepool": true}
		for _, pair := range strings.Split(*np.Labels, ",") {
			parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("karpenter nodepool '%s': invalid label '%s', use format key=value", np.Name, pair)
			}
			if reservedPoolLabels[parts[0]] {
				return fmt.Errorf("karpenter nodepool '%s': '%s' is a Convox-reserved label key and cannot be overridden", np.Name, parts[0])
			}
		}
	}

	if np.Taints != nil && *np.Taints != "" {
		if strings.Contains(*np.Taints, `"`) {
			return fmt.Errorf("karpenter nodepool '%s': taint keys and values must not contain double quotes", np.Name)
		}
		validEffects := map[string]bool{"NoSchedule": true, "PreferNoSchedule": true, "NoExecute": true}
		for _, t := range strings.Split(*np.Taints, ",") {
			t = strings.TrimSpace(t)
			colonParts := strings.SplitN(t, ":", 2)
			if len(colonParts) != 2 || colonParts[0] == "" {
				return fmt.Errorf("karpenter nodepool '%s': invalid taint '%s', use format key=value:Effect or key:Effect", np.Name, t)
			}
			if !validEffects[colonParts[1]] {
				return fmt.Errorf("karpenter nodepool '%s': invalid taint effect '%s' (must be NoSchedule, PreferNoSchedule, or NoExecute)", np.Name, colonParts[1])
			}
		}
	}

	return nil
}

type KarpenterNodePools []KarpenterNodePoolConfigParam

func (knp KarpenterNodePools) Validate() error {
	nameMap := map[string]bool{}
	for i := range knp {
		if err := knp[i].Validate(); err != nil {
			return err
		}
		if nameMap[knp[i].Name] {
			return fmt.Errorf("duplicate karpenter nodepool name: %s", knp[i].Name)
		}
		nameMap[knp[i].Name] = true
	}
	return nil
}

type AdditionalNodeGroups []NodeGroupConfigParam

func (an AdditionalNodeGroups) Validate() error {
	idCnt := 0
	idMap := map[int]bool{}
	for i := range an {
		if err := an[i].Validate(); err != nil {
			return err
		}
		if an[i].Id != nil {
			idCnt++
			if idMap[*an[i].Id] {
				return fmt.Errorf("duplicate node group id is found: %d", *an[i].Id)
			}
		}
	}

	if idCnt > 0 && idCnt != len(an) {
		return fmt.Errorf("some node groups missing id property")
	}

	if idCnt == 0 {
		for i := range an {
			an[i].Id = options.Int(i)
		}
	}
	return nil
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr := make([]int, lb+1)
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			curr[j] = ins
			if del < curr[j] {
				curr[j] = del
			}
			if sub < curr[j] {
				curr[j] = sub
			}
		}
		prev = curr
	}
	return prev[lb]
}

func suggestParam(key string, known map[string]bool) string {
	best, bestDist := "", 4
	for k := range known {
		if d := levenshtein(key, k); d < bestDist {
			best, bestDist = k, d
		}
	}
	return best
}

func validateAndMutateParams(params map[string]string, provider string, currentParams map[string]string, force bool) error {
	// Install-only params — these define infrastructure that cannot be changed
	// after rack creation without catastrophic consequences (network recreation,
	// subnet destruction, cluster replacement). Block ANY post-install modification.
	//
	// MAINTENANCE: Keep in sync with immutableRackParams in console3
	// api/controller/cli.go. Add any new TF variable here if changing it
	// post-install would destroy or recreate core infrastructure.
	installOnlyParams := map[string]bool{
		"high_availability":   true,
		"private":             true,
		"cidr":                true,
		"vpc_id":              true,
		"internet_gateway_id": true,
		"private_subnets_ids": true,
		"public_subnets_ids":  true,
		"availability_zones":  true,
		"region":              true,
		"access_id":           true,
		"secret_key":          true,
		"token":               true,
	}

	for k := range params {
		if installOnlyParams[k] {
			return fmt.Errorf("param '%s' can only be set during rack installation", k)
		}
	}

	for k := range params {
		if k == "" {
			return fmt.Errorf("parameter name cannot be empty")
		}
	}

	// V2 racks use PascalCase params (e.g. "HighAvailability", "BuildMemory").
	// V3 racks use snake_case (e.g. "high_availability", "build_node_type").
	// Detect V2 by checking if any currentParam key starts with an uppercase letter.
	// Only check when provider is a known V3 provider — unknown providers (e.g. test
	// fixtures) skip the known-key check via known==nil but should still run
	// other validation (empty-value, terraform_update_timeout, etc.).
	isV3Rack := true
	if providerKnownParams[provider] != nil {
		for k := range currentParams {
			if len(k) > 0 && k[0] >= 'A' && k[0] <= 'Z' {
				isV3Rack = false
				break
			}
		}
	}

	if !isV3Rack {
		return nil
	}

	known := providerKnownParams[provider]

	if !force {
		// Managed params are set automatically by `convox rack update` (image, release,
		// k8s_version, etc.). Block direct modification unless --force is used.
		for k := range params {
			if managedParams[k] && (known == nil || known[k]) {
				return fmt.Errorf("param '%s' is managed internally — to update it use 'convox rack update'. Use --force to override", k)
			}
		}

		// Spellcheck: reject unknown keys with a suggestion when close to a known param.
		if known != nil {
			for k := range params {
				// Karpenter params on non-AWS providers are caught later with a
				// provider-specific error — don't shadow that with "unknown parameter".
				if provider != "aws" && (strings.HasPrefix(k, "karpenter_") || k == "additional_karpenter_nodepools_config") {
					continue
				}
				if !known[k] {
					msg := fmt.Sprintf("unknown parameter '%s' for %s provider", k, provider)
					if suggestion := suggestParam(k, known); suggestion != "" {
						msg += fmt.Sprintf("\n       Did you mean '%s'?", suggestion)
					}
					msg += "\n       Run 'sudo convox update' to get the latest parameter support, or use --force to override."
					return fmt.Errorf("%s", msg)
				}
			}
		}
	} else {
		// --force bypasses managed and unknown-key guards, but still warn about managed params.
		for k := range params {
			if managedParams[k] && (known == nil || known[k]) {
				fmt.Fprintf(os.Stderr, "WARNING: '%s' is a managed parameter — setting it directly may break your rack.\n", k)
			}
		}
	}

	// Only these params accept empty strings — empty means "clear this setting."
	// ALL other params require explicit values. Empty strings for non-clearable
	// params cause TF type errors, silent default reversion, or state divergence
	// between what convox rack params shows and what TF actually applies.
	//
	// MAINTENANCE: When adding a new TF variable where users should be able to
	// clear a previously-set value with param="", add it here AND to preserveEmpty
	// in pkg/rack/terraform.go. Default is REJECT — only add if clearing makes sense.
	clearableParams := map[string]bool{
		// Labels/taints — clear means "remove all"
		"karpenter_node_labels":       true,
		"karpenter_node_taints":       true,
		"karpenter_build_node_labels": true,
		// Instance restrictions — clear means "no restriction"
		"karpenter_instance_families":       true,
		"karpenter_instance_sizes":          true,
		"karpenter_build_instance_families": true,
		"karpenter_build_instance_sizes":    true,
		// Schedule — clear means "disable schedule" (must be paired)
		"schedule_rack_scale_down": true,
		"schedule_rack_scale_up":   true,
		// Tags — clear means "remove all custom tags"
		"tags": true,
		// SSL — clear means "use defaults"
		"ssl_ciphers":   true,
		"ssl_protocols": true,
		// Optional overrides — clear means "use auto/default"
		"build_node_type":         true,
		"key_pair_name":           true,
		"nginx_additional_config": true,
		// Credentials — clear means "remove auth"
		"docker_hub_username": true,
		"docker_hub_password": true,
		// Logging — clear means "stop shipping"
		"syslog": true,
		// Domain — clear means "use auto-managed"
		"convox_rack_domain": true,
		// Custom launch scripts — clear means "remove"
		"user_data":     true,
		"user_data_url": true,
		// Feature gates — clear means "disable all"
		"api_feature_gates": true,
		// Private EKS — cleared by console during mode changes
		"private_eks_host": true,
		"private_eks_user": true,
		"private_eks_pass": true,
		// JSON config — normalized to base64 later in this function
		"additional_node_groups_config":         true,
		"additional_build_groups_config":        true,
		"additional_karpenter_nodepools_config": true,
		"karpenter_config":                      true,
	}

	for k, v := range params {
		if strings.TrimSpace(v) == "" && !clearableParams[k] {
			return fmt.Errorf("param '%s' requires an explicit value (omit to keep current)", k)
		}
	}

	srdown, srup := params["schedule_rack_scale_down"], params["schedule_rack_scale_up"]
	if (srdown == "" || srup == "") && (srdown != "" || srup != "") {
		return errors.New("to schedule your rack to turn on/off you need both schedule_rack_scale_down and schedule_rack_scale_up parameters")
	}

	// format: "key1=val1,key2=val2" — empty string clears all tags
	if tags, has := params["tags"]; has && tags != "" {
		tList := strings.Split(tags, ",")
		for _, p := range tList {
			if len(strings.SplitN(p, "=", 2)) != 2 {
				return errors.New("invalid value for tags param")
			}
		}
	}

	if v, has := params["terraform_update_timeout"]; has {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid value for terraform_update_timeout: must be a valid duration (e.g., '2h', '90m', '2h30m'): %s", err)
		}
		if d <= 0 {
			return errors.New("invalid value for terraform_update_timeout: must be a positive duration")
		}
	}

	if v, has := params["imds_http_tokens"]; has && v != "" {
		if v != "optional" && v != "required" {
			return fmt.Errorf("param 'imds_http_tokens' must be 'optional' or 'required'")
		}
	}

	if v, has := params["node_capacity_type"]; has && v != "" {
		lower := strings.ToLower(v)
		if lower != "on_demand" && lower != "spot" && lower != "mixed" {
			return fmt.Errorf("param 'node_capacity_type' must be 'on_demand', 'spot', or 'mixed'")
		}
	}

	if v, has := params["access_log_retention_in_days"]; has && v != "" {
		if _, err := strconv.Atoi(v); err != nil {
			return fmt.Errorf("param 'access_log_retention_in_days' must be an integer")
		}
	}

	// Reject karpenter_* params for non-AWS racks
	if provider != "aws" {
		for k := range params {
			if strings.HasPrefix(k, "karpenter_") || k == "additional_karpenter_nodepools_config" {
				return fmt.Errorf("karpenter parameters are only supported for AWS racks")
			}
		}
	}

	// karpenter_auth_mode and karpenter_enabled are type=string in TF (not bool).
	// Reject junk values — only "true" or "false" are valid.
	for _, boolishParam := range []string{"karpenter_auth_mode", "karpenter_enabled"} {
		if v, ok := params[boolishParam]; ok && v != "" && v != "true" && v != "false" {
			return fmt.Errorf("param '%s' must be 'true' or 'false'", boolishParam)
		}
	}

	// karpenter_auth_mode: one-way migration (cannot be disabled once enabled)
	if params["karpenter_auth_mode"] == "false" && currentParams["karpenter_auth_mode"] == "true" {
		return fmt.Errorf("karpenter_auth_mode cannot be disabled once enabled (AWS EKS access config migration is one-way)")
	}

	// karpenter_enabled=true requires karpenter_auth_mode=true — either already
	// applied or being set in the same call.
	if params["karpenter_enabled"] == "true" && currentParams["karpenter_enabled"] != "true" {
		if currentParams["karpenter_auth_mode"] != "true" && params["karpenter_auth_mode"] != "true" {
			return fmt.Errorf("karpenter_enabled=true requires karpenter_auth_mode=true.\n  Either include both: convox rack params set karpenter_auth_mode=true karpenter_enabled=true\n  Or set karpenter_auth_mode=true first and wait for the update to complete")
		}
	}

	// ecr_docker_hub_cache=true requires docker_hub_username and docker_hub_password —
	// either already applied or being set in the same call. AWS ECR pull-through
	// cache rules require authenticated Docker Hub credentials.
	if params["ecr_docker_hub_cache"] == "true" && currentParams["ecr_docker_hub_cache"] != "true" {
		hasUsername := (currentParams["docker_hub_username"] != "" || params["docker_hub_username"] != "")
		hasPassword := (currentParams["docker_hub_password"] != "" || params["docker_hub_password"] != "")
		if !hasUsername || !hasPassword {
			return fmt.Errorf("ecr_docker_hub_cache=true requires docker_hub_username and docker_hub_password.\n  Set all three: convox rack params set ecr_docker_hub_cache=true docker_hub_username=USER docker_hub_password=TOKEN\n  Or set docker_hub_username and docker_hub_password first")
		}
	}

	if v, has := params["karpenter_node_volume_type"]; has && v != "" {
		validVolTypes := map[string]bool{"gp2": true, "gp3": true, "io1": true, "io2": true}
		if !validVolTypes[v] {
			return fmt.Errorf("param 'karpenter_node_volume_type' must be gp2, gp3, io1, or io2")
		}
	}

	// Karpenter parameter validation
	// When enabling Karpenter, temporarily inject current params for re-validation so stale
	// invalid values saved during a previous karpenter_enabled=false call are caught.
	// Injected keys are removed after validation to avoid sending them to UpdateParams.
	var karpenterInjectedKeys []string
	if params["karpenter_enabled"] == "true" {
		karpenterRevalidateKeys := []string{
			"karpenter_arch",
			"karpenter_capacity_types", "karpenter_cpu_limit", "karpenter_memory_limit_gb",
			"karpenter_consolidate_after", "karpenter_build_consolidate_after",
			"karpenter_node_expiry", "karpenter_disruption_budget_nodes",
			"karpenter_build_capacity_types", "karpenter_build_cpu_limit",
			"karpenter_build_memory_limit_gb", "karpenter_node_taints",
			"karpenter_node_labels", "karpenter_build_node_labels",
		}
		for _, rk := range karpenterRevalidateKeys {
			if _, inCall := params[rk]; !inCall {
				if cv, exists := currentParams[rk]; exists && cv != "" {
					params[rk] = cv
					karpenterInjectedKeys = append(karpenterInjectedKeys, rk)
				}
			}
		}
	}
	if v, ok := params["karpenter_arch"]; ok && v != "" {
		for _, arch := range strings.Split(v, ",") {
			arch = strings.TrimSpace(arch)
			if arch != "amd64" && arch != "arm64" {
				return fmt.Errorf("invalid karpenter architecture: %s (must be amd64 or arm64)", arch)
			}
		}
	}

	if v, ok := params["karpenter_capacity_types"]; ok && v != "" {
		for _, ct := range strings.Split(v, ",") {
			ct = strings.TrimSpace(ct)
			if ct != "on-demand" && ct != "spot" {
				return fmt.Errorf("invalid karpenter capacity type: %s (must be on-demand or spot)", ct)
			}
		}
	}

	if v, ok := params["karpenter_cpu_limit"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("karpenter_cpu_limit must be a positive integer")
		}
	}

	if v, ok := params["karpenter_memory_limit_gb"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("karpenter_memory_limit_gb must be a positive integer")
		}
	}

	durationRe := regexp.MustCompile(`^\d+[smh]$`)
	for _, dk := range []string{"karpenter_consolidate_after", "karpenter_build_consolidate_after"} {
		if v, ok := params[dk]; ok && v != "" {
			if !durationRe.MatchString(v) {
				return fmt.Errorf("%s must be a duration like 30s, 5m, or 1h", dk)
			}
		}
	}

	if v, ok := params["karpenter_node_expiry"]; ok && v != "" {
		expiryRe := regexp.MustCompile(`^\d+h$`)
		if v != "Never" && !expiryRe.MatchString(v) {
			return fmt.Errorf("karpenter_node_expiry must be a duration like 720h or Never")
		}
	}

	if v, ok := params["karpenter_disruption_budget_nodes"]; ok && v != "" {
		budgetRe := regexp.MustCompile(`^\d+%?$`)
		if !budgetRe.MatchString(v) {
			return fmt.Errorf("karpenter_disruption_budget_nodes must be a number or percentage (e.g. 10%%)")
		}
	}

	if v, ok := params["karpenter_build_capacity_types"]; ok && v != "" {
		for _, ct := range strings.Split(v, ",") {
			ct = strings.TrimSpace(ct)
			if ct != "on-demand" && ct != "spot" {
				return fmt.Errorf("invalid karpenter build capacity type: %s (must be on-demand or spot)", ct)
			}
		}
	}

	if v, ok := params["karpenter_build_cpu_limit"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("karpenter_build_cpu_limit must be a positive integer")
		}
	}

	if v, ok := params["karpenter_build_memory_limit_gb"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("karpenter_build_memory_limit_gb must be a positive integer")
		}
	}

	// Validate karpenter_node_taints format: key=value:Effect (same check as KarpenterNodePoolConfigParam.Validate)
	if v, ok := params["karpenter_node_taints"]; ok && v != "" {
		if strings.Contains(v, `"`) {
			return fmt.Errorf("karpenter_node_taints: taint keys and values must not contain double quotes")
		}
		validEffects := map[string]bool{"NoSchedule": true, "PreferNoSchedule": true, "NoExecute": true}
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			colonParts := strings.SplitN(t, ":", 2)
			if len(colonParts) != 2 || colonParts[0] == "" {
				return fmt.Errorf("invalid karpenter_node_taints entry '%s', use format key=value:Effect or key:Effect", t)
			}
			if !validEffects[colonParts[1]] {
				return fmt.Errorf("invalid karpenter_node_taints effect '%s' (must be NoSchedule, PreferNoSchedule, or NoExecute)", colonParts[1])
			}
		}
	}

	// Reject Convox-reserved label keys and double quotes in label values
	reservedWorkloadLabels := map[string]bool{"convox.io/nodepool": true}
	if v, ok := params["karpenter_node_labels"]; ok && v != "" {
		if strings.Contains(v, `"`) {
			return fmt.Errorf("karpenter_node_labels: label keys and values must not contain double quotes")
		}
		for _, pair := range strings.Split(v, ",") {
			k := strings.TrimSpace(strings.SplitN(pair, "=", 2)[0])
			if reservedWorkloadLabels[k] {
				return fmt.Errorf("karpenter_node_labels: '%s' is a Convox-reserved label key and cannot be overridden", k)
			}
		}
	}

	reservedBuildLabels := map[string]bool{"convox-build": true, "convox.io/nodepool": true}
	if v, ok := params["karpenter_build_node_labels"]; ok && v != "" {
		if strings.Contains(v, `"`) {
			return fmt.Errorf("karpenter_build_node_labels: label keys and values must not contain double quotes")
		}
		for _, pair := range strings.Split(v, ",") {
			k := strings.TrimSpace(strings.SplitN(pair, "=", 2)[0])
			if reservedBuildLabels[k] {
				return fmt.Errorf("karpenter_build_node_labels: '%s' is a Convox-reserved label key and cannot be overridden", k)
			}
		}
	}

	// Remove temporarily-injected currentParams keys so they aren't sent to UpdateParams
	for _, rk := range karpenterInjectedKeys {
		delete(params, rk)
	}

	// Normalize empty values for JSON config params — users clear with param= or param=""
	// but TF needs valid base64-encoded JSON (empty array or empty object).
	for _, k := range []string{"additional_node_groups_config", "additional_build_groups_config", "additional_karpenter_nodepools_config"} {
		if v, ok := params[k]; ok && (v == "" || v == `""`) {
			params[k] = base64.StdEncoding.EncodeToString([]byte("[]"))
		}
	}
	if v, ok := params["karpenter_config"]; ok && (v == "" || v == `""`) {
		params["karpenter_config"] = base64.StdEncoding.EncodeToString([]byte("{}"))
	}

	ngKeys := []string{"additional_node_groups_config", "additional_build_groups_config"}
	for _, k := range ngKeys {
		if params[k] != "" {
			var err error
			cfgData := []byte(params[k])
			if strings.HasSuffix(params[k], ".json") {
				cfgData, err = os.ReadFile(params[k])
				if err != nil {
					return fmt.Errorf("invalid param '%s' value, failed to read the file: %s", k, err)
				}
			} else if !strings.HasPrefix(params[k], "[") {
				data, err := base64.StdEncoding.DecodeString(params[k])
				if err != nil {
					return fmt.Errorf("invalid param '%s' value: %s", k, err)
				}

				cfgData = data
			}

			nCfgs := AdditionalNodeGroups{}
			if err := json.Unmarshal(cfgData, &nCfgs); err != nil {
				return err
			}

			// Preserve existing ids from currentParams when new config omits them.
			// This prevents Terraform for_each key changes (destroy+create cycles)
			// when a user modifies config without specifying ids.
			hasNilId := false
			for _, ng := range nCfgs {
				if ng.Id == nil {
					hasNilId = true
					break
				}
			}
			if hasNilId && currentParams[k] != "" {
				var existingData []byte
				if decoded, err := base64.StdEncoding.DecodeString(currentParams[k]); err == nil {
					existingData = decoded
				} else {
					existingData = []byte(currentParams[k])
				}
				var existing AdditionalNodeGroups
				if err := json.Unmarshal(existingData, &existing); err == nil {
					for i := range nCfgs {
						if nCfgs[i].Id != nil {
							continue
						}
						if i < len(existing) {
							if existing[i].Id != nil {
								nCfgs[i].Id = existing[i].Id
							} else {
								// Legacy entry without id — use 0-based index to match Terraform's idx default
								nCfgs[i].Id = options.Int(i)
							}
						}
					}
				}
			}

			if err := nCfgs.Validate(); err != nil {
				return err
			}

			sort.Slice(nCfgs, func(i, j int) bool {
				if nCfgs[i].Id == nil || nCfgs[j].Id == nil {
					return true
				}
				return *nCfgs[i].Id < *nCfgs[j].Id
			})

			data, err := json.Marshal(nCfgs)
			if err != nil {
				return fmt.Errorf("failed to process params '%s': %s", k, err)
			}
			params[k] = base64.StdEncoding.EncodeToString(data)
		}
	}

	// Preflight check: warn if enabling Karpenter with non-dedicated additional node groups
	karpenterBeingEnabled := params["karpenter_enabled"] == "true"
	karpenterAlreadyEnabled := currentParams["karpenter_enabled"] == "true" && params["karpenter_enabled"] != "false"
	if karpenterBeingEnabled || karpenterAlreadyEnabled {
		// Use the config from this call if provided, otherwise check current state
		ngConfig := params["additional_node_groups_config"]
		if ngConfig == "" {
			ngConfig = currentParams["additional_node_groups_config"]
		}
		if ngConfig != "" {
			var ngCfgData []byte
			if decoded, err := base64.StdEncoding.DecodeString(ngConfig); err == nil {
				ngCfgData = decoded
			} else {
				ngCfgData = []byte(ngConfig)
			}
			var ngs AdditionalNodeGroups
			if err := json.Unmarshal(ngCfgData, &ngs); err == nil {
				for _, ng := range ngs {
					if ng.Dedicated == nil || !*ng.Dedicated {
						return fmt.Errorf("karpenter_enabled=true requires all additional node groups to have dedicated=true to prevent scheduling overlap with Karpenter workload nodes; update your additional_node_groups_config to set dedicated=true on all groups, or include the updated config in the same call")
					}
				}
			}
		}
	}

	// Karpenter custom NodePools config (same pattern as additional_node_groups_config)
	if params["additional_karpenter_nodepools_config"] != "" {
		k := "additional_karpenter_nodepools_config"
		var err error
		cfgData := []byte(params[k])
		if strings.HasSuffix(params[k], ".json") {
			cfgData, err = os.ReadFile(params[k])
			if err != nil {
				return fmt.Errorf("invalid param '%s' value, failed to read the file: %s", k, err)
			}
		} else if !strings.HasPrefix(params[k], "[") {
			data, err := base64.StdEncoding.DecodeString(params[k])
			if err != nil {
				return fmt.Errorf("invalid param '%s' value: %s", k, err)
			}
			cfgData = data
		}

		npCfgs := KarpenterNodePools{}
		if err := json.Unmarshal(cfgData, &npCfgs); err != nil {
			return fmt.Errorf("invalid karpenter nodepools config: %s", err)
		}

		if err := npCfgs.Validate(); err != nil {
			return err
		}

		sort.Slice(npCfgs, func(i, j int) bool {
			return npCfgs[i].Name < npCfgs[j].Name
		})

		data, err := json.Marshal(npCfgs)
		if err != nil {
			return fmt.Errorf("failed to process param '%s': %s", k, err)
		}
		params[k] = base64.StdEncoding.EncodeToString(data)
	}

	// karpenter_config — validate JSON and block protected fields
	if params["karpenter_config"] != "" {
		k := "karpenter_config"
		var err error
		cfgData := []byte(params[k])
		if strings.HasSuffix(params[k], ".json") {
			cfgData, err = os.ReadFile(params[k])
			if err != nil {
				return fmt.Errorf("invalid param '%s' value, failed to read the file: %s", k, err)
			}
		} else if !strings.HasPrefix(params[k], "{") {
			data, err := base64.StdEncoding.DecodeString(params[k])
			if err != nil {
				return fmt.Errorf("invalid param '%s' value: %s", k, err)
			}
			cfgData = data
		}

		const maxKarpenterConfigSize = 64 * 1024 // 64 KB
		if len(cfgData) > maxKarpenterConfigSize {
			return fmt.Errorf("karpenter_config exceeds maximum size of 64KB (%d bytes)", len(cfgData))
		}

		var config map[string]interface{}
		if err := json.Unmarshal(cfgData, &config); err != nil {
			return fmt.Errorf("invalid karpenter_config JSON: %s", err)
		}

		// Only nodePool and ec2NodeClass sections are allowed
		for key := range config {
			if key != "nodePool" && key != "ec2NodeClass" {
				return fmt.Errorf("karpenter_config: unknown top-level key '%s' (allowed: nodePool, ec2NodeClass)", key)
			}
		}

		// Block protected EC2NodeClass fields that would break infrastructure
		if ec2Val, exists := config["ec2NodeClass"]; exists {
			ec2, ok := ec2Val.(map[string]interface{})
			if !ok {
				return fmt.Errorf("karpenter_config: ec2NodeClass must be a JSON object")
			}
			blocked := []string{"role", "instanceProfile", "subnetSelectorTerms", "securityGroupSelectorTerms"}
			for _, field := range blocked {
				if _, exists := ec2[field]; exists {
					return fmt.Errorf("karpenter_config: ec2NodeClass.%s is managed by Convox and cannot be overridden", field)
				}
			}

			// Block reserved tag keys in ec2NodeClass.tags (Name/Rack are forced by Convox)
			if tagsVal, exists := ec2["tags"]; exists {
				tags, ok := tagsVal.(map[string]interface{})
				if !ok {
					return fmt.Errorf("karpenter_config: ec2NodeClass.tags must be a JSON object")
				}
				reservedTags := []string{"name", "rack"}
				for tagKey := range tags {
					if common.ContainsInStringSlice(reservedTags, strings.ToLower(tagKey)) {
						return fmt.Errorf("karpenter_config: reserved tag key '%s' is not allowed in ec2NodeClass.tags (managed by Convox)", tagKey)
					}
				}
			}
		}

		// Block protected NodePool fields
		if npVal, exists := config["nodePool"]; exists {
			np, ok := npVal.(map[string]interface{})
			if !ok {
				return fmt.Errorf("karpenter_config: nodePool must be a JSON object")
			}
			if tmplVal, exists := np["template"]; exists {
				tmpl, ok := tmplVal.(map[string]interface{})
				if !ok {
					return fmt.Errorf("karpenter_config: nodePool.template must be a JSON object")
				} else {
					// Block reserved labels in nodePool.template.metadata.labels
					if metaVal, exists := tmpl["metadata"]; exists {
						meta, ok := metaVal.(map[string]interface{})
						if !ok {
							return fmt.Errorf("karpenter_config: nodePool.template.metadata must be a JSON object")
						}
						if labelsVal, exists := meta["labels"]; exists {
							labels, ok := labelsVal.(map[string]interface{})
							if !ok {
								return fmt.Errorf("karpenter_config: nodePool.template.metadata.labels must be a JSON object")
							}
							reservedConfigLabels := []string{"convox.io/nodepool"}
							for labelKey := range labels {
								for _, reserved := range reservedConfigLabels {
									if labelKey == reserved {
										return fmt.Errorf("karpenter_config: '%s' is a Convox-reserved label and cannot be overridden in nodePool.template.metadata.labels", labelKey)
									}
								}
							}
						}
					}
					if specVal, exists := tmpl["spec"]; exists {
						spec, ok := specVal.(map[string]interface{})
						if !ok {
							return fmt.Errorf("karpenter_config: nodePool.template.spec must be a JSON object")
						} else {
							if _, exists := spec["nodeClassRef"]; exists {
								return fmt.Errorf("karpenter_config: nodePool.template.spec.nodeClassRef is managed by Convox and cannot be overridden")
							}
							if reqVal, exists := spec["requirements"]; exists {
								reqs, ok := reqVal.([]interface{})
								if !ok {
									return fmt.Errorf("karpenter_config: nodePool.template.spec.requirements must be a JSON array")
								}
								if len(reqs) == 0 {
									return fmt.Errorf("karpenter_config: nodePool.template.spec.requirements must not be empty (Karpenter needs at least one requirement to provision nodes)")
								}
							}
						}
					}
				}
			}
		}

		// Re-encode to base64 for storage (normalized)
		data, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to process karpenter_config: %s", err)
		}
		params[k] = base64.StdEncoding.EncodeToString(data)
	}

	return nil
}

func Rack(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	i := c.Info()

	i.Add("Name", s.Name)
	i.Add("Provider", s.Provider)

	if s.Region != "" {
		i.Add("Region", s.Region)
	}

	if s.Domain != "" {
		if ri := s.Outputs["DomainInternal"]; ri != "" {
			i.Add("Router", fmt.Sprintf("%s (external)\n%s (internal)", s.Domain, ri))
		} else {
			i.Add("Router", s.Domain)
		}
	}

	if s.RouterInternal != "" {
		i.Add("RouterInternal", s.RouterInternal)
	}

	if nlb := s.Outputs["NLBHost"]; nlb != "" {
		var eips []string
		for _, k := range []string{"NLBEIP0", "NLBEIP1", "NLBEIP2"} {
			if v := s.Outputs[k]; v != "" {
				eips = append(eips, v)
			}
		}
		if len(eips) > 0 {
			i.Add("NLB", fmt.Sprintf("%s (%s)", nlb, strings.Join(eips, ", ")))
		} else {
			i.Add("NLB", nlb)
		}
	}
	if nlbi := s.Outputs["NLBInternalHost"]; nlbi != "" {
		i.Add("NLB Internal", nlbi)
	}

	i.Add("Status", s.Status)
	i.Add("Version", s.Version)

	return i.Print()
}

func RackAccess(rack sdk.Interface, c *stdcli.Context) error {
	data, err := c.SettingRead("current")
	if err != nil {
		return err
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(data), &attrs); err != nil {
		return err
	}

	rData, err := rack.SystemGet()
	if err != nil {
		return err
	}

	role, ok := c.Value("role").(string)
	if !ok {
		return fmt.Errorf("role is required")
	}

	duration, ok := c.Value("duration-in-hours").(int)
	if !ok {
		return fmt.Errorf("duration is required")
	}

	jwtTk, err := rack.SystemJwtToken(structs.SystemJwtOptions{
		Role:           options.String(role),
		DurationInHour: options.String(strconv.Itoa(duration)),
	})
	if err != nil {
		return err
	}

	return c.Writef("RACK_URL=https://jwt:%s@%s\n", jwtTk.Token, rData.RackDomain)
}

func RackAccessKeyRotate(rack sdk.Interface, c *stdcli.Context) error {
	_, err := rack.SystemJwtSignKeyRotate()
	if err != nil {
		return err
	}

	return c.OK()
}

func RackInstall(_ sdk.Interface, c *stdcli.Context) error {
	slug := c.Arg(0)
	name := c.Arg(1)
	args := c.Args[2:]
	version := c.String("version")
	runtime := c.String("runtime")

	if !provider.Valid(slug) {
		return fmt.Errorf("unknown provider: %s", slug)
	}
	var parts []string
	if runtime != "" {
		parts = strings.Split(name, "/")
		name = parts[1]
	}

	if err := checkRackNameRegex(name); err != nil {
		return err
	}

	opts := argsToOptions(args)

	if c.Bool("prepare") {
		opts["release"] = version

		md := &rack.Metadata{
			Provider: slug,
			Vars:     opts,
		}

		if _, err := rack.Create(c, name, md); err != nil {
			return err
		}

		return nil
	}

	if runtime != "" {
		name = parts[0] + "/" + parts[1]
	}

	if err := rack.Install(c, slug, name, version, runtime, opts); err != nil {
		return err
	}

	if runtime != "" {
		c.Writef("Convox Rack installation initiated. Check the progress on the Console Racks page if desired. \n")
	}

	if _, err := rack.Current(c); err != nil {
		if _, err := rack.Switch(c, name); err != nil {
			return err
		}
	}

	return nil
}

func RackKarpenterCleanup(rack sdk.Interface, c *stdcli.Context) error {
	c.Startf("Cleaning up Karpenter nodes")

	if err := rack.KarpenterCleanup(); err != nil {
		return err
	}

	return c.OK()
}

func RackKubeconfig(_ sdk.Interface, c *stdcli.Context) error {
	r, err := rack.Current(c)
	if err != nil {
		return err
	}

	ep, err := r.Endpoint()
	if err != nil {
		return err
	}

	pw, _ := ep.User.Password()

	data := strings.TrimSpace(fmt.Sprintf(`
apiVersion: v1
clusters:
- cluster:
    server: %s://%s/kubernetes/
  name: rack
contexts:
- context:
    cluster: rack
    user: convox
  name: convox@rack
current-context: convox@rack
kind: Config
users:
- name: convox
  user:
    username: "%s"
    password: "%s"
	`, ep.Scheme, ep.Host, ep.User.Username(), pw))

	fmt.Println(data)

	return nil
}

func RackLogs(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.LogsOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	if c.Bool("no-follow") {
		opts.Follow = options.Bool(false)
	}

	opts.Prefix = options.Bool(true)

	r, err := rack.SystemLogs(opts)
	if err != nil {
		return err
	}

	io.Copy(c, r)

	return nil
}

func RackMv(_ sdk.Interface, c *stdcli.Context) error {
	from := c.Arg(0)
	to := c.Arg(1)

	movedToConsole, toRackName := false, to
	parts := strings.SplitN(to, "/", 2)
	if len(parts) == 2 {
		movedToConsole = true
		toRackName = parts[1]
	}

	fromRackName := from
	fparts := strings.SplitN(from, "/", 2)
	if len(fparts) == 2 {
		fromRackName = fparts[1]
	}

	if fromRackName != toRackName {
		return fmt.Errorf("rack name must remain same")
	}

	c.Startf("moving rack <rack>%s</rack> to <rack>%s</rack>", from, to)

	fr, err := rack.Load(c, from)
	if err != nil {
		return err
	}

	md, err := fr.Metadata()
	if err != nil {
		return err
	}

	if !md.Deletable {
		return fmt.Errorf("rack %s has dependencies and can not be moved", from)
	}

	md, err = fr.Metadata()
	if err != nil {
		return err
	}

	if _, err := rack.Create(c, to, md); err != nil {
		return err
	}

	if err := fr.Delete(); err != nil {
		return err
	}

	if movedToConsole {
		ci := c.Info()
		ci.Add("Attention!", "Login in the console and attach a runtime integration to the rack")
	}

	return c.OK()
}

func RackParams(_ sdk.Interface, c *stdcli.Context) error {
	var (
		groupFilter   map[string]bool
		resolvedGroup string
	)
	if groupInput := c.String("group"); groupInput != "" {
		resolved, rerr := resolveGroup(groupInput)
		if rerr != nil {
			return rerr
		}
		resolvedGroup = resolved
		groupFilter = paramGroups[resolved]
	}

	r, err := rack.Current(c)
	if err != nil {
		return err
	}

	params, err := r.Parameters()
	if err != nil {
		return err
	}

	keys := []string{}

	for k := range params {
		keys = append(keys, k)
	}

	b64Keys := []string{"additional_node_groups_config", "additional_build_groups_config", "additional_karpenter_nodepools_config", "karpenter_config"}
	for _, k := range b64Keys {
		if params[k] != "" {
			v, err := base64.StdEncoding.DecodeString(params[k])
			if err == nil {
				params[k] = string(v)
			}
		}
	}

	sort.Strings(keys)

	shouldMask := !c.Bool("reveal") && IsTerminalFn(c)

	i := c.Info()
	rowsAdded := 0

	for _, k := range keys {
		if groupFilter != nil && !groupFilter[k] {
			continue
		}
		v := params[k]
		if shouldMask && sensitiveParams[k] && v != "" {
			v = "**********"
		}
		i.Add(k, v)
		rowsAdded++
	}

	if err := i.Print(); err != nil {
		return err
	}

	if groupFilter != nil && rowsAdded == 0 {
		// Write via stdcli's captured writer so test harnesses observe the
		// NOTICE. The underlying stdcli.Writer.Stderr defaults to os.Stderr,
		// so this is byte-identical in production; only test buffers differ.
		fmt.Fprintf(c.Writer().Stderr, "NOTICE: no params in group '%s' for this rack\n", resolvedGroup)
	}

	return nil
}

func RackParamsSet(_ sdk.Interface, c *stdcli.Context) error {
	r, err := rack.Current(c)
	if err != nil {
		return err
	}

	c.Startf("Updating parameters")

	currentParams, err := r.Parameters()
	if err != nil {
		return err
	}

	params := argsToOptions(c.Args)
	force, _ := c.Value("force").(bool)
	if err := validateAndMutateParams(params, r.Provider(), currentParams, force); err != nil {
		return err
	}

	if err := r.UpdateParams(params); err != nil {
		return err
	}

	return c.OK()
}

func RackPs(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.SystemProcessesOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	ps, err := rack.SystemProcesses(opts)
	if err != nil {
		return err
	}

	t := c.Table("ID", "APP", "SERVICE", "STATUS", "RELEASE", "STARTED", "COMMAND")

	for _, p := range ps {
		t.AddRow(p.Id, p.App, p.Name, p.Status, p.Release, common.Ago(p.Started), p.Command)
	}

	return t.Print()
}

func RackReleases(rack sdk.Interface, c *stdcli.Context) error {
	rs, err := rack.SystemReleases()
	if err != nil {
		return err
	}

	t := c.Table("VERSION", "UPDATED")

	for _, r := range rs {
		t.AddRow(r.Id, common.Ago(r.Created))
	}

	return t.Print()
}

func RackRuntimes(rack sdk.Interface, c *stdcli.Context) error {
	data, err := c.SettingRead("current")
	if err != nil {
		return err
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(data), &attrs); err != nil {
		return err
	}

	rs, err := rack.Runtimes(attrs["name"])
	if err != nil {
		return err
	}

	t := c.Table("ID", "TITLE")
	for _, r := range rs {
		t.AddRow(r.Id, r.Title)
	}

	return t.Print()
}

func RackRuntimeAttach(rack sdk.Interface, c *stdcli.Context) error {
	data, err := c.SettingRead("current")
	if err != nil {
		return err
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(data), &attrs); err != nil {
		return err
	}

	if err := rack.RuntimeAttach(attrs["name"], structs.RuntimeAttachOptions{
		Runtime: aws.String(c.Arg(0)),
	}); err != nil {
		return err
	}

	return c.OK()
}

func RackScale(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	var opts structs.SystemUpdateOptions
	update := false

	if v, ok := c.Value("count").(int); ok {
		opts.Count = options.Int(v)
		update = true
	}

	if v, ok := c.Value("type").(string); ok {
		opts.Type = options.String(v)
		update = true
	}

	if update {
		c.Startf("Scaling rack")

		if err := rack.SystemUpdate(opts); err != nil {
			return err
		}

		return c.OK()
	}

	i := c.Info()

	i.Add("Autoscale", s.Parameters["Autoscale"])
	i.Add("Count", fmt.Sprintf("%d", s.Count))
	i.Add("Status", s.Status)
	i.Add("Type", s.Type)

	return i.Print()
}

func RackSync(_ sdk.Interface, c *stdcli.Context) error {
	r, err := rack.Current(c)
	if err != nil {
		return err
	}

	data, err := c.SettingRead("current")
	if err != nil {
		return err
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(data), &attrs); err != nil {
		return err
	}

	if attrs["type"] == "console" {
		m, err := r.Metadata()
		if err != nil {
			return err
		}

		if m.State == nil { // v2 racks don't have a state file
			err := r.Sync()
			if err != nil {
				return err
			}

			return c.OK()
		}
	}

	return fmt.Errorf("sync is only supported for console managed v2 racks")
}

func RackUninstall(_ sdk.Interface, c *stdcli.Context) error {
	name := c.Arg(0)

	r, err := rack.Match(c, name)
	if err != nil {
		return err
	}

	if err := r.Uninstall(); err != nil {
		return err
	}

	return nil
}

func RackUpdate(_ sdk.Interface, c *stdcli.Context) error {
	r, err := rack.Current(c)
	if err != nil {
		return err
	}

	cl, err := r.Client()
	if err != nil {
		return err
	}

	s, err := cl.SystemGet()
	if err != nil {
		return err
	}

	currentVersion := s.Version
	newVersion := c.Arg(0)

	// disable downgrabe from minor version for v3 rack
	if strings.HasPrefix(currentVersion, "3.") && strings.HasPrefix(newVersion, "3.") &&
		!strings.Contains(currentVersion, "rc") && !strings.Contains(newVersion, "rc") {
		curv, err := strconv.Atoi(strings.Split(currentVersion, ".")[1])
		if err != nil {
			return err
		}

		newv, err := strconv.Atoi(strings.Split(newVersion, ".")[1])
		if err != nil {
			return err
		}
		if newv < curv {
			return fmt.Errorf("Downgrade from minor version is not supported for v3 rack. Contact the support.")
		}
	}

	force, _ := c.Value("force").(bool)
	if err := r.UpdateVersion(newVersion, force); err != nil {
		return err
	}

	return nil
}
