package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/provider/aws/provisioner/rds"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

const (
	StateFinalizer = "convox.com/rds-provisioner"
	StateDataKey   = "state"
)

func (p *Provider) CreateRdsResourceStateId(app string, resourceName string) string {
	resourceName = nameFilter(resourceName)
	return fmt.Sprintf("%s-r%sr-%s", resourceName, p.RackName, app)
}

func (p *Provider) ParseAppNameFromStateId(id string) (string, error) {
	parts := strings.Split(id, fmt.Sprintf("-r%sr-", p.RackName))
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid state id")
	}
	return parts[1], nil
}

func (p *Provider) ParseResourceNameFromStateId(id string) (string, error) {
	parts := strings.Split(id, fmt.Sprintf("-r%sr-", p.RackName))
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid state id")
	}
	return parts[0], nil
}

func (p *Provider) SaveState(id string, data []byte) error {
	app, err := p.ParseAppNameFromStateId(id)
	if err != nil {
		return err
	}

	_, err = p.CreateOrPatchSecret(p.ctx, metav1.ObjectMeta{
		Name:      id,
		Namespace: p.AppNamespace(app),
	}, func(s *corev1.Secret) *corev1.Secret {
		if !hasStateFinalizer(s.Finalizers) {
			s.Finalizers = append(s.Finalizers, StateFinalizer)
		}

		s.Labels = map[string]string{
			"rack":       p.RackName,
			"system":     "convox",
			"provsioner": "rds",
			"type":       "state",
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
	app, err := p.ParseAppNameFromStateId(id)
	if err != nil {
		return nil, err
	}

	s, err := p.Cluster.CoreV1().Secrets(p.AppNamespace(app)).Get(p.ctx, id, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil, fmt.Errorf("state not found")
		}
		return nil, err
	}

	data, has := s.Data[StateDataKey]
	if !has {
		return nil, fmt.Errorf("state not found")
	}

	return data, err
}

func (p *Provider) SendStateLog(id, message string) error {
	app, err := p.ParseAppNameFromStateId(id)
	if err != nil {
		return err
	}

	return p.systemLog(app, "state", time.Now(), fmt.Sprintf("rds resource: %s", message))
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

func (p *Provider) MapToRdsParameter(rdsType, app string, params map[string]string) map[string]string {
	out := map[string]string{
		rds.ParamEngine:    strings.TrimPrefix(rdsType, "rds-"),
		rds.ParamVPC:       common.CoalesceString(params["vpc"], p.VpcID),
		rds.ParamSubnetIds: common.CoalesceString(params["subnets"], p.SubnetIDs),
	}

	titleCaser := cases.Title(language.Und, cases.NoLower)

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
			out[titleCaser.String(k)] = v
		}
	}

	if strings.HasPrefix(out[rds.ParamSourceDBInstanceIdentifier], "#convox.resources.") {
		rName := strings.TrimPrefix(out[rds.ParamSourceDBInstanceIdentifier], "#convox.resources.")
		out[rds.ParamSourceDBInstanceIdentifier] = p.CreateRdsResourceStateId(app, rName)
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
