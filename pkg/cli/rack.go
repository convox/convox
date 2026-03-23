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

func init() {
	register("rack", "get information about the rack", watch(Rack), stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagWatchInterval},
		Validate: stdcli.Args(0),
	})

	register("rack access", "get rack access credential", RackAccess, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.StringFlag("role", "", "access role: read or write"),
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
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack params set", "set rack parameters", RackParamsSet, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
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
			an[i].Id = options.Int(i + 1)
		}
	}
	return nil
}

func validateAndMutateParams(params map[string]string, provider string, currentParams map[string]string) error {
	if params["high_availability"] != "" {
		return errors.New("the high_availability parameter is only supported during rack installation")
	}

	srdown, srup := params["ScheduleRackScaleDown"], params["ScheduleRackScaleUp"]
	if (srdown == "" || srup == "") && (srdown != "" || srup != "") {
		return errors.New("to schedule your rack to turn on/off you need both ScheduleRackScaleDown and ScheduleRackScaleUp parameters")
	}

	// format: "key1=val1,key2=val2"
	if tags, has := params["tags"]; has {
		tList := strings.Split(tags, ",")
		for _, p := range tList {
			if len(strings.Split(p, "=")) != 2 {
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

	// Reject karpenter_* params for non-AWS racks
	if provider != "aws" {
		for k := range params {
			if strings.HasPrefix(k, "karpenter_") || k == "additional_karpenter_nodepools_config" {
				return fmt.Errorf("karpenter parameters are only supported for AWS racks")
			}
		}
	}

	// karpenter_auth_mode: one-way migration (cannot be disabled once enabled)
	if params["karpenter_auth_mode"] == "false" && currentParams["karpenter_auth_mode"] == "true" {
		return fmt.Errorf("karpenter_auth_mode cannot be disabled once enabled (AWS EKS access config migration is one-way)")
	}

	// karpenter_enabled=true requires karpenter_auth_mode to already be true OR be set to true in the same call
	if params["karpenter_enabled"] == "true" {
		if currentParams["karpenter_auth_mode"] != "true" && params["karpenter_auth_mode"] != "true" {
			return fmt.Errorf("karpenter_enabled=true requires karpenter_auth_mode=true (set it first, or include karpenter_auth_mode=true in the same call)")
		}
	}

	// Karpenter parameter validation
	if params["karpenter_enabled"] == "true" {
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

	ngKeys := []string{"additional_node_groups_config", "additional_build_groups_config"}
	for _, k := range ngKeys {
		if params[k] != "" {
			v, err := base64.StdEncoding.DecodeString(params[k])
			if err == nil {
				params[k] = string(v)
			}
		}
	}

	sort.Strings(keys)

	i := c.Info()

	for _, k := range keys {
		i.Add(k, params[k])
	}

	return i.Print()
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
	if err := validateAndMutateParams(params, r.Provider(), currentParams); err != nil {
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
