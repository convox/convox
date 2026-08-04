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
	"github.com/convox/convox/pkg/structs"
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

	AnnotationUninstalledAt = "convox.com/uninstalled-at"

	MetaAppKey      = "convox-x-app"
	MetaRackKey     = "convox-x-rack"
	MetaResourceKey = "convox-x-resource"
	MetaRdsTypeKey  = "convox-x-rds-type"
	MetaTidKey      = "convox-x-tid"
)

var (
	tempRdsEventLogStore = &tempStateLogStorage{
		lock:      sync.Mutex{},
		s:         map[string][]string{},
		threshold: 50,
	}

	awsResourceMetaOptions = []string{"class", "durable", "instance", "nodes", "storage", "version"}
)

func generateResourceStateId(rack, tid, app, resourceName string) string {
	if tid != "" {
		return fmt.Sprintf("%s-%s-%s", resourceName, tid, app)
	}
	return fmt.Sprintf("%s-r%sr-%s", resourceName, rack, app)
}

// CreateAwsResourceStateId returns the state id for a resource. A tenant id is not
// parseable, so it also gets a labeled secret the state lookups read it back from.
func (p *Provider) CreateAwsResourceStateId(tid, app, resourceName string) (string, error) {
	id := generateResourceStateId(p.Name, tid, app, resourceName)
	if tid == "" {
		return id, nil
	}

	ns := p.tidNamespace(tid, app)

	cur, err := p.Cluster.CoreV1().Secrets(ns).Get(p.ctx, id, metav1.GetOptions{})
	if err == nil {
		if cur.DeletionTimestamp != nil {
			return "", structs.ErrBadRequest("resource %s is still being deleted, promote again once it is gone", resourceName)
		}
		return id, nil
	}
	if !kerr.IsNotFound(err) {
		return "", err
	}

	_, err = p.Cluster.CoreV1().Secrets(ns).Create(p.ctx, &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
			Labels: map[string]string{
				"rack":     p.RackName,
				"system":   "convox",
				"app":      app,
				"resource": resourceName,
				"tid":      tid,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil && !kerr.IsAlreadyExists(err) {
		return "", fmt.Errorf("failed to create state secret for resource %s: %s", resourceName, err)
	}

	return id, nil
}

type AwsStateIdInfo struct {
	App          string
	ResourceName string
	Tid          string
}

func (p *Provider) GetInfoFromAwsResourceStateId(id string) (*AwsStateIdInfo, error) {
	if parts := strings.Split(id, fmt.Sprintf("-r%sr-", p.Name)); len(parts) == 2 {
		return &AwsStateIdInfo{App: parts[1], ResourceName: parts[0]}, nil
	}

	s, err := p.awsResourceStateSecret(id)
	if err != nil {
		return nil, err
	}

	return &AwsStateIdInfo{
		App:          s.Labels["app"],
		ResourceName: s.Labels["resource"],
		Tid:          s.Labels["tid"],
	}, nil
}

// The provisioner storage callbacks run on the base provider, with no tenant
// context, so a tenant id can only be resolved by name across namespaces.
func (p *Provider) awsResourceStateSecret(id string) (*corev1.Secret, error) {
	if app, err := p.ParseAppNameFromAwsResourceStateId(id); err == nil {
		s, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(app)).Get(p.ctx, id, metav1.GetOptions{})
		if err != nil {
			if kerr.IsNotFound(err) {
				return nil, structs.ErrNotFound("state not found")
			}
			return nil, err
		}
		return s, nil
	}

	sList, err := p.Cluster.CoreV1().Secrets(corev1.NamespaceAll).List(p.ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", id),
		LabelSelector: "system=convox",
	})
	if err != nil {
		return nil, err
	}

	for i := range sList.Items {
		if sList.Items[i].Name == id {
			return &sList.Items[i], nil
		}
	}

	return nil, structs.ErrNotFound("state not found")
}

func (p *Provider) AwsResourceTags(tid, app, resourceName string) map[string]string {
	tags := map[string]string{
		"rack":     p.RackName,
		"system":   "convox",
		"app":      app,
		"resource": resourceName,
	}
	if tid != "" {
		tags["tid"] = tid
	}
	return tags
}

// awsResourceMeta records what a resource was asked for next to its state. With no
// option template to say which options exist, only the descriptive ones are taken:
// the rest of the manifest options may be credentials.
func (p *Provider) awsResourceMeta(tid, app string, r manifest.Resource, requested map[string]string) map[string]string {
	meta := map[string]string{}

	if requested != nil {
		for k, v := range requested {
			meta[k] = v
		}
	} else {
		for _, k := range awsResourceMetaOptions {
			if v := r.Options[k]; v != "" {
				meta[k] = v
			}
		}
	}

	meta[MetaAppKey] = app
	meta[MetaRackKey] = p.RackName
	meta[MetaResourceKey] = r.Name
	meta[MetaTidKey] = tid

	return meta
}

func (p *Provider) ParseAppNameFromAwsResourceStateId(id string) (string, error) {
	parts := strings.Split(id, fmt.Sprintf("-r%sr-", p.Name))
	if len(parts) != 2 {
		return "", structs.ErrBadRequest("invalid state id")
	}
	return parts[1], nil
}

func (p *Provider) ParseResourceNameFromAwsResourceStateId(id string) (string, error) {
	parts := strings.Split(id, fmt.Sprintf("-r%sr-", p.Name))
	if len(parts) != 2 {
		return "", structs.ErrBadRequest("invalid state id")
	}
	return parts[0], nil
}

func (p *Provider) SaveState(id string, data []byte, provisioner string, meta map[string]string) error {
	app, resourceName, tid := meta[MetaAppKey], meta[MetaResourceKey], meta[MetaTidKey]

	if app == "" {
		info, err := p.GetInfoFromAwsResourceStateId(id)
		if err != nil {
			return err
		}
		app, resourceName, tid = info.App, info.ResourceName, info.Tid
	}

	ns := p.AppNamespace(app)
	if tid != "" {
		ns = p.tidNamespace(tid, app)
	}

	_, err := p.CreateOrPatchSecret(p.ctx, metav1.ObjectMeta{
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
			"tid":         tid,
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

func (p *Provider) GetState(id string) ([]byte, error) {
	s, err := p.awsResourceStateSecret(id)
	if err != nil {
		return nil, err
	}

	data, has := s.Data[StateDataKey]
	if !has {
		return nil, structs.ErrNotFound("state not found")
	}

	return data, nil
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
		_ = p.systemLog(tid, app, "state", time.Now(), msg)
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

// MapToRdsParameterAndMeta maps only the options the rack's template allows.
func (p *Provider) MapToRdsParameterAndMeta(tid, rdsType, app string, r manifest.Resource) (map[string]string, map[string]string, error) {
	allowed, requested, err := p.filterRdsOptionsForTemplate(strings.TrimPrefix(rdsType, "rds-"), r.Options)
	if err != nil {
		return nil, nil, err
	}

	return p.MapToRdsParameter(rdsType, app, allowed), p.awsResourceMeta(tid, app, r, requested), nil
}

func (p *Provider) MapToRdsParameter(rdsType, app string, params map[string]string) map[string]string {
	out := map[string]string{
		rds.ParamEngine:    strings.TrimPrefix(rdsType, "rds-"),
		rds.ParamVPC:       common.CoalesceString(params["vpc"], p.VpcID),
		rds.ParamSubnetIds: common.CoalesceString(params["subnets"], p.SubnetIDs),
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
	return filtered
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
			s.Annotations[AnnotationUninstalledAt] = time.Now().UTC().Format(time.RFC3339)
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
			s.Annotations[AnnotationUninstalledAt] = time.Now().UTC().Format(time.RFC3339)
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

// The resource template and the atom dependency both key off the prefixed type,
// so a provider alias has to carry it even though the manifest did not.
func awsResourceType(prefix, rType string) string {
	if strings.HasPrefix(rType, prefix) {
		return rType
	}
	return prefix + rType
}

func (p *Provider) validateAwsProviderResource(r manifest.Resource) error {
	if !r.IsAwsProvider() {
		return nil
	}

	if !p.FeatureGates[options.FeatureGateRDSTemplateConfig] {
		return structs.ErrBadRequest("resource %s: provider %s is not supported on this rack", r.Name, r.Provider)
	}

	if !r.IsRds() && !r.IsElastiCache() {
		return structs.ErrBadRequest("resource %s: provider %s is not supported for type %s", r.Name, r.Provider, r.Type)
	}

	return nil
}

type rdsOption struct {
	ParamName            string
	AllowedValues        []string
	AllowedMaximum       *int
	AllowedMinimum       *int
	Default              *string
	MapAllowedToOriginal map[string]string
}

func (ro *rdsOption) ValidateAndMapValue(val string) (string, error) {
	if len(ro.AllowedValues) > 0 && !common.ContainsInStringSlice(ro.AllowedValues, val) {
		return "", fmt.Errorf("value '%s' is not allowed", val)
	}

	if ro.AllowedMinimum != nil || ro.AllowedMaximum != nil {
		intVal, err := strconv.Atoi(val)
		if err != nil {
			return "", fmt.Errorf("value '%s' is not a valid integer", val)
		}
		if ro.AllowedMinimum != nil && intVal < *ro.AllowedMinimum {
			return "", fmt.Errorf("value '%s' is below the allowed minimum %d", val, *ro.AllowedMinimum)
		}
		if ro.AllowedMaximum != nil && intVal > *ro.AllowedMaximum {
			return "", fmt.Errorf("value '%s' is above the allowed maximum %d", val, *ro.AllowedMaximum)
		}
	}

	if orig, has := ro.MapAllowedToOriginal[val]; has {
		return orig, nil
	}

	return val, nil
}

func (p *Provider) filterRdsOptionsForTemplate(rdsEngine string, opts map[string]string) (map[string]string, map[string]string, error) {
	cmName := options.GetFeatureGateValue(options.FeatureGateRDSTemplateConfig)

	cm, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Get(p.ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load db option template %s: %s", cmName, err)
	}

	data, has := cm.Data["config"]
	if !has {
		return nil, nil, fmt.Errorf("db option template %s has no config", cmName)
	}

	template := map[string][]rdsOption{}
	if err := json.Unmarshal([]byte(data), &template); err != nil {
		return nil, nil, fmt.Errorf("failed to parse db option template %s: %s", cmName, err)
	}

	engineOptions, has := template[rdsEngine]
	if !has {
		return nil, nil, structs.ErrBadRequest("db type '%s' is not supported", rdsEngine)
	}

	requested := map[string]string{}

	for _, o := range engineOptions {
		if v := opts[o.ParamName]; v != "" {
			requested[o.ParamName] = v
		} else if o.Default != nil {
			requested[o.ParamName] = *o.Default
		}
	}

	// storage follows the class rather than being chosen separately
	requested["class"] = strings.ToLower(requested["class"])
	requested["storage"] = requested["class"]

	// vpc and subnets are deliberately absent: a curated resource goes where the
	// rack says, not where the manifest asks
	out := map[string]string{}

	// declared order so the option a release is rejected for is deterministic
	for _, o := range engineOptions {
		v, has := requested[o.ParamName]
		if !has {
			continue
		}

		mapped, err := o.ValidateAndMapValue(v)
		if err != nil {
			return nil, nil, structs.ErrBadRequest("db options %s: %s", o.ParamName, err)
		}
		out[o.ParamName] = mapped
	}

	return out, requested, nil
}
