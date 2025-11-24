package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/provider/aws/provisioner/elasticache"
	"github.com/convox/convox/provider/aws/provisioner/rds"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

const (
	StateFinalizer = "convox.com/rds-provisioner"
	StateDataKey   = "state"

	MetaAppKey         = "convox-x-app"
	MetaTidKey         = "convox-x-tid"
	MetaRackKey        = "convox-x-rack"
	MetaResourceKey    = "convox-x-resource"
	MetaProvisionerKey = "convox-x-provisioner"
	MetaRdsTypeKey     = "convox-x-rds-type"
)

var (
	tempRdsEventLogStore = &tempStateLogStorage{
		lock:      sync.Mutex{},
		s:         map[string][]string{},
		threshold: 50,
	}
)

func generateResourceStateId(rack, tid, app, resourceName string) string {
	separator := fmt.Sprintf("-r%sr-", rack)
	if tid != "" {
		separator = fmt.Sprintf("-%s-", tid)
	}
	return fmt.Sprintf("%s%s%s", resourceName, separator, app)
}

func (p *Provider) CreateAwsResourceStateId(tid, app string, resourceName string) (string, error) {
	stateId := generateResourceStateId(p.Name, tid, app, resourceName)

	_, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(app)).Create(p.ctx, &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: stateId,
			Labels: map[string]string{
				"rack":     p.RackName,
				"system":   "convox",
				"app":      app,
				"resource": resourceName,
				"tid":      p.ContextTID(),
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerr.IsAlreadyExists(err) {
		return "", fmt.Errorf("Error creating state secret for %s: %s", app, err)
	}
	return stateId, nil
}

type AwsStateIdInfo struct {
	App          string
	ResourceName string
	Tid          string
	Rack         string
}

func (p *Provider) GetInfoFromAwsResourceStateId(id string) (*AwsStateIdInfo, error) {
	separator := fmt.Sprintf("-r%sr-", p.Name)
	parts := strings.Split(id, separator)
	if len(parts) == 2 {
		return &AwsStateIdInfo{
			App:          parts[1],
			ResourceName: parts[0],
			Rack:         p.Name,
		}, nil
	}

	resp, err := p.Cluster.CoreV1().Secrets(corev1.NamespaceAll).List(p.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", id),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get state secret: %s", err)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("state secret not found")
	}

	return &AwsStateIdInfo{
		App:          resp.Items[0].Labels["app"],
		ResourceName: resp.Items[0].Labels["resource"],
		Tid:          resp.Items[0].Labels["tid"],
		Rack:         resp.Items[0].Labels["rack"],
	}, nil
}

func (p *Provider) AwsResourceTags(app string, resourceName string) map[string]string {
	return map[string]string{
		"rack":     p.Name,
		"system":   "convox",
		"app":      app,
		"resource": resourceName,
		"tid":      p.ContextTID(),
	}
}

// it is not provider customer context aware function
func (p *Provider) SaveState(id string, data []byte, provisioner string, meta map[string]string) error {
	if meta == nil {
		meta = map[string]string{}
	}

	var err error
	app := meta[MetaAppKey]
	if app == "" {
		info, err := p.GetInfoFromAwsResourceStateId(id)
		if err != nil {
			return err
		}
		app = info.App
	}

	resourceName := meta[MetaResourceKey]
	if resourceName == "" {
		info, err := p.GetInfoFromAwsResourceStateId(id)
		if err != nil {
			return err
		}
		resourceName = info.ResourceName
	}

	ns := p.AppNamespace(app)
	if meta[MetaTidKey] != "" {
		ns = p.TidNamespace(meta[MetaTidKey], app)
	}

	_, err = p.CreateOrPatchSecret(p.ctx, metav1.ObjectMeta{
		Name:      id,
		Namespace: ns,
	}, func(s *corev1.Secret) *corev1.Secret {
		if !hasStateFinalizer(s.Finalizers) {
			s.Finalizers = append(s.Finalizers, StateFinalizer)
		}

		s.Labels = map[string]string{
			"rack":        p.RackName,
			"system":      "convox",
			"provisioner": provisioner,
			"type":        "state",
			"app":         app,
			"resource":    resourceName,
			"tid":         meta[MetaTidKey],
		}

		s.Data = map[string][]byte{
			StateDataKey: data,
		}
		return s
	}, metav1.PatchOptions{
		FieldManager: "convox",
	})

	return err
}

// it is not provider customer context aware function
func (p *Provider) GetState(id string) ([]byte, error) {
	sList, err := p.Cluster.CoreV1().Secrets(corev1.NamespaceAll).List(p.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", id),
	})
	if err != nil {
		return nil, err
	}
	if len(sList.Items) == 0 {
		return nil, fmt.Errorf("state not found")
	}

	data, has := sList.Items[0].Data[StateDataKey]
	if !has {
		return nil, fmt.Errorf("state not found")
	}

	return data, err
}

func (p *Provider) SendStateLog(id, message string) error {
	info, err := p.GetInfoFromAwsResourceStateId(id)
	if err != nil {
		return err
	}

	tempRdsEventLogStore.Add(info.Tid, info.App, fmt.Sprintf("resource %s: %s", info.ResourceName, message))
	return nil
}

func (p *Provider) FlushStateLog(tid, app string) {
	logList := tempRdsEventLogStore.Get(tid, app)
	tempRdsEventLogStore.Reset(tid, app)
	for _, msg := range logList {
		p.systemLog(tid, app, "state", time.Now(), msg)
	}
}

func (p *Provider) ListRdsStateForApp(app string) ([]string, error) {
	resp, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(app)).List(p.ctx, metav1.ListOptions{
		LabelSelector: "system=convox,type=state",
	})
	if err != nil {
		return nil, err
	}

	stateIds := []string{}
	for i := range resp.Items {
		if resp.Items[i].Labels["provisioner"] == "" || resp.Items[i].Labels["provisioner"] == rds.ProvisionerName {
			stateIds = append(stateIds, resp.Items[i].Name)
		}
	}
	return stateIds, nil
}

func (p *Provider) ListElasticacheStateForApp(app string) ([]string, error) {
	resp, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(app)).List(p.ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("system=convox,type=state,provisioner=%s", elasticache.ProvisionerName),
	})
	if err != nil {
		return nil, err
	}

	stateIds := []string{}
	for i := range resp.Items {
		stateIds = append(stateIds, resp.Items[i].Name)
	}
	return stateIds, nil
}

func (p *Provider) CreateOrPatchSecret(ctx context.Context, meta metav1.ObjectMeta, transform func(*corev1.Secret) *corev1.Secret, opts metav1.PatchOptions) (*corev1.Secret, error) {
	cur, err := p.Cluster.CoreV1().Secrets(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		p.logger.Logf("Creating Scret %s/%s.", meta.Namespace, meta.Name)
		out, err := p.Cluster.CoreV1().Secrets(meta.Namespace).Create(ctx, transform(&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}), metav1.CreateOptions{
			DryRun:       opts.DryRun,
			FieldManager: opts.FieldManager,
		})
		return out, err
	} else if err != nil {
		return nil, err
	}
	return p.PatchSecret(ctx, cur, transform, opts)
}

func (p *Provider) PatchSecret(ctx context.Context, cur *corev1.Secret, transform func(*corev1.Secret) *corev1.Secret, opts metav1.PatchOptions) (*corev1.Secret, error) {
	return p.PatchSecretObject(ctx, cur, transform(cur.DeepCopy()), opts)
}

func (p *Provider) PatchSecretObject(ctx context.Context, cur, mod *corev1.Secret, opts metav1.PatchOptions) (*corev1.Secret, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, corev1.Secret{})
	if err != nil {
		return nil, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, nil
	}
	p.logger.Logf("Patching Secret %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	return p.Cluster.CoreV1().Secrets(cur.Namespace).Patch(ctx, cur.Name, types.StrategicMergePatchType, patch, opts)
}

func (p *Provider) MapToRdsParameterAndMeta(rdsType, app string, r manifest.Resource) (map[string]string, map[string]string, error) {
	params := r.Options

	out := map[string]string{
		rds.ParamEngine:    strings.TrimPrefix(rdsType, "rds-"),
		rds.ParamVPC:       common.CoalesceString(params["vpc"], p.VpcID),
		rds.ParamSubnetIds: common.CoalesceString(params["subnets"], p.SubnetIDs),
	}

	params, meta, err := p.filterRDSOptionsForTemplate(strings.TrimPrefix(rdsType, "rds-"), params)
	if err != nil {
		return nil, nil, err
	}

	meta[MetaAppKey] = app
	meta[MetaResourceKey] = r.Name
	meta[MetaRdsTypeKey] = strings.TrimPrefix(rdsType, "rds-")
	meta[MetaRackKey] = p.RackName
	if p.ContextTID() != "" {
		meta[MetaTidKey] = p.ContextTID()
	}

	for k, v := range params {
		switch k {
		case "encrypted": // for rack v2 rds param backward compatibility
			out[rds.ParamStorageEncrypted] = v
		case "deletionProtection": // for rack v2 rds param backward compatibility
			out[rds.ParamDeletionProtection] = v
		case "durable": // for rack v2 rds param backward compatibility
			out[rds.ParamMultiAZ] = v
		case "iops": // for rack v2 rds param backward compatibility
			out[rds.ParamIops] = v
		case "storage": // for rack v2 rds param backward compatibility
			out[rds.ParamAllocatedStorage] = v
		case "preferredBackupWindow": // for rack v2 rds param backward compatibility
			out[rds.ParamPreferredBackupWindow] = v
		case "backupRetentionPeriod": // for rack v2 rds param backward compatibility
			out[rds.ParamBackupRetentionPeriod] = v
		case "readSourceDB": // for rack v2 rds param backward compatibility
			out[rds.ParamSourceDBInstanceIdentifier] = v
		case "class", "instance":
			out[rds.ParamDBInstanceClass] = v
		case "version":
			out[rds.ParamEngineVersion] = v
		default:
			for _, pKey := range rds.ParametersNameList() {
				if strings.EqualFold(k, pKey) {
					out[pKey] = v
				}
			}
		}
	}

	if strings.HasPrefix(out[rds.ParamSourceDBInstanceIdentifier], "#convox.resources.") {
		rName := strings.TrimPrefix(out[rds.ParamSourceDBInstanceIdentifier], "#convox.resources.")
		out[rds.ParamSourceDBInstanceIdentifier] = generateResourceStateId(p.Name, p.ContextTID(), app, rName)
	}

	allowedParamList := map[string]struct{}{}
	for _, pKey := range rds.ParametersNameList() {
		allowedParamList[pKey] = struct{}{}
	}

	filtered := map[string]string{}
	for k, v := range out {
		if _, has := allowedParamList[k]; has {
			filtered[k] = v
		}
	}
	return filtered, meta, nil
}

func (p *Provider) MapToElasticacheParameter(cacheType, app string, params map[string]string) map[string]string {
	out := map[string]string{
		elasticache.ParamEngine:    strings.TrimPrefix(cacheType, "elasticache-"),
		elasticache.ParamVPC:       common.CoalesceString(params["vpc"], p.VpcID),
		elasticache.ParamSubnetIds: common.CoalesceString(params["subnets"], p.SubnetIDs),
	}

	for k, v := range params {
		switch k {
		case "deletionProtection":
			out[elasticache.ParamDeletionProtection] = v
		case "durable":
			out[elasticache.ParamAutomaticFailoverEnabled] = v
		case "nodes":
			if out[elasticache.ParamEngine] == "redis" {
				out[elasticache.ParamNumCacheClusters] = v
			} else {
				out[elasticache.ParamNumCacheNodes] = v
			}
		case "encrypted":
			out[elasticache.ParamAtRestEncryptionEnabled] = v
		case "class", "instance":
			out[elasticache.ParamCacheNodeType] = v
		case "version":
			out[elasticache.ParamEngineVersion] = v
		case "password":
			out[elasticache.ParamAuthToken] = v
		default:
			for _, pKey := range elasticache.ParametersNameList() {
				if strings.EqualFold(k, pKey) {
					out[pKey] = v
				}
			}
		}
	}

	allowedParamList := map[string]struct{}{}
	for _, pKey := range elasticache.ParametersNameList() {
		allowedParamList[pKey] = struct{}{}
	}

	filtered := map[string]string{}
	for k, v := range out {
		if _, has := allowedParamList[k]; has {
			filtered[k] = v
		}
	}
	return filtered
}

func (p *Provider) uninstallRdsAssociatedWithStateSecret(stateSecret *corev1.Secret) error {
	if err := p.RdsProvisioner.Uninstall(stateSecret.Name); err != nil {
		return err
	}

	_, err := p.PatchSecret(p.ctx, stateSecret, func(s *corev1.Secret) *corev1.Secret {
		if hasStateFinalizer(s.Finalizers) {
			newFinalizers := []string{}
			for _, fn := range s.Finalizers {
				if fn != StateFinalizer {
					newFinalizers = append(newFinalizers, fn)
				}
			}
			s.Finalizers = newFinalizers
			if s.Annotations == nil {
				s.Annotations = map[string]string{}
			}
			s.Annotations["convox.com/uninstalled-at"] = time.Now().UTC().Format(time.RFC3339)
		}
		return s
	}, metav1.PatchOptions{})

	return err
}

func (p *Provider) uninstallElaticacheAssociatedWithStateSecret(stateSecret *corev1.Secret) error {
	if err := p.ElasticacheProvisioner.Uninstall(stateSecret.Name); err != nil {
		return err
	}

	_, err := p.PatchSecret(p.ctx, stateSecret, func(s *corev1.Secret) *corev1.Secret {
		if hasStateFinalizer(s.Finalizers) {
			newFinalizers := []string{}
			for _, fn := range s.Finalizers {
				if fn != StateFinalizer {
					newFinalizers = append(newFinalizers, fn)
				}
			}
			s.Finalizers = newFinalizers
			if s.Annotations == nil {
				s.Annotations = map[string]string{}
			}
			s.Annotations["convox.com/uninstalled-at"] = time.Now().UTC().Format(time.RFC3339)
		}
		return s
	}, metav1.PatchOptions{})

	return err
}

func hasStateFinalizer(finalizers []string) bool {
	for _, fn := range finalizers {
		if fn == StateFinalizer {
			return true
		}
	}
	return false
}

type RDSOptions struct {
	ParamName            string
	AllowedValues        []string
	AllowedMaximum       *int
	AllowedMinimum       *int
	Default              *string
	MapAllowedToOriginal map[string]string
}

// Check if a value is allowed based on AllowedValues
func (ro *RDSOptions) CheckAllowedValue(val string) bool {
	if len(ro.AllowedValues) == 0 {
		return true
	}
	for _, allowed := range ro.AllowedValues {
		if val == allowed {
			return true
		}
	}
	return false
}

// Check if a value is within AllowedMinimum and AllowedMaximum (for integer values)
func (ro *RDSOptions) CheckAllowedRange(val int) bool {
	if ro.AllowedMinimum != nil && val < *ro.AllowedMinimum {
		return false
	}
	if ro.AllowedMaximum != nil && val > *ro.AllowedMaximum {
		return false
	}
	return true
}

// Map an allowed value to its original value using MapAllowedToOriginal
func (ro *RDSOptions) mapAllowedToOriginalValue(val string) string {
	if ro.MapAllowedToOriginal == nil {
		return val
	}
	if orig, ok := ro.MapAllowedToOriginal[val]; ok {
		return orig
	}
	return val
}

func (ro *RDSOptions) HasDefaultValue() bool {
	return ro.Default != nil
}

func (ro *RDSOptions) GetDefaultValue() (string, error) {
	if ro.Default != nil {
		return *ro.Default, nil
	}
	return "", fmt.Errorf("no default value set")
}

func (ro *RDSOptions) ValidateAndMapValue(val string) (string, error) {
	if !ro.CheckAllowedValue(val) {
		return "", fmt.Errorf("value '%s' is not allowed", val)
	}
	if ro.AllowedMinimum != nil || ro.AllowedMaximum != nil {
		intVal, err := strconv.Atoi(val)
		if err != nil {
			return "", fmt.Errorf("value '%s' is not a valid integer", val)
		}
		if !ro.CheckAllowedRange(intVal) {
			return "", fmt.Errorf("value '%s' is out of allowed range (%d-%d)", val, ro.AllowedMinimum, ro.AllowedMaximum)
		}
	}

	mappedVal := ro.mapAllowedToOriginalValue(val)
	return mappedVal, nil
}

func (p *Provider) filterRDSOptionsForTemplate(rdsEngine string, opts map[string]string) (map[string]string, map[string]string, error) {
	if !options.GetFeatureGates()[options.FeatureGateRDSTemplateConfig] {
		return opts, nil, nil
	}

	cmName := options.GetFeatureGateValue(options.FeatureGateRDSTemplateConfig)
	cm, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Get(p.ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	parameterOptionsJson, ok := cm.Data["config"]
	if !ok {
		return nil, nil, fmt.Errorf("config not found in configmap %s", cmName)
	}

	parameterOptions := map[string][]RDSOptions{}
	err = json.Unmarshal([]byte(parameterOptionsJson), &parameterOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal rds_template_basic_config.json: %v", err)
	}

	meta := map[string]string{}

	returned := map[string]string{}

	parameterOptionsRdsType, ok := parameterOptions[rdsEngine]
	if !ok {
		return nil, nil, fmt.Errorf("rds type '%s' not supported for basic template parameters", rdsEngine)
	}

	parameterOptionsRdsTypeMap := map[string]RDSOptions{}
	for _, opt := range parameterOptionsRdsType {
		parameterOptionsRdsTypeMap[opt.ParamName] = opt
		// set default values if not provided
		if opt.HasDefaultValue() && opts[opt.ParamName] == "" {
			val, err := opt.GetDefaultValue()
			if err != nil {
				return nil, nil, err
			}
			opts[opt.ParamName] = val
		}
	}

	opts["class"] = strings.ToLower(opts["class"])

	opts["storage"] = opts["class"] // storage is fixed to class

	for k, v := range opts {
		if opts, ok := parameterOptionsRdsTypeMap[k]; ok {
			returned[k], err = opts.ValidateAndMapValue(v)
			if err != nil {
				return nil, nil, fmt.Errorf("db options: %s", err)
			}
		}

		meta[k] = v
	}

	return returned, meta, nil
}
