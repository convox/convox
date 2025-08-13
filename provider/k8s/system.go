package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	flowcontrolv1 "k8s.io/api/flowcontrol/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	amv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

const ConvoxJwtSecretName = "convox-jwt-key"

func (p *Provider) SystemGet() (*structs.System, error) {
	status := "running"

	s := &structs.System{
		Domain:     fmt.Sprintf("router.%s", p.Domain),
		Name:       p.RackName,
		Provider:   p.Provider,
		RackDomain: fmt.Sprintf("api.%s", p.Domain),
		Status:     status,
		Version:    p.Version,
	}

	if p.DomainInternal != "" {
		s.RouterInternal = fmt.Sprintf("router.%s", p.DomainInternal)
	}

	return s, nil
}

func (p *Provider) SystemInstall(w io.Writer, opts structs.SystemInstallOptions) (string, error) {
	return "", errors.WithStack(fmt.Errorf("unimplemented"))
}

func (p *Provider) SystemJwtSignKey() (string, error) {
	s, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get(context.TODO(), ConvoxJwtSecretName, am.GetOptions{})
	if err != nil && !kerr.IsNotFound(err) {
		return "", err
	}

	if s == nil || s.Data["signKey"] == nil {
		return p.updateJwtSignKey()
	}

	return base64.StdEncoding.EncodeToString(s.Data["signKey"]), nil
}

func (p *Provider) SystemJwtSignKeyRotate() (string, error) {
	key, err := p.updateJwtSignKey()
	if err != nil {
		return "", err
	}

	sObj := &appsv1.DeploymentApplyConfiguration{
		TypeMetaApplyConfiguration: amv1.TypeMetaApplyConfiguration{
			Kind:       options.String("Deployment"),
			APIVersion: options.String("apps/v1"),
		},
		ObjectMetaApplyConfiguration: &amv1.ObjectMetaApplyConfiguration{
			Name: options.String("api"),
		},
		Spec: &appsv1.DeploymentSpecApplyConfiguration{
			Template: &corev1.PodTemplateSpecApplyConfiguration{
				ObjectMetaApplyConfiguration: &amv1.ObjectMetaApplyConfiguration{
					Annotations: map[string]string{
						"convox.com/restartAt": time.Now().String(),
					},
				},
			},
		},
	}
	_, err = p.Cluster.AppsV1().Deployments(p.Namespace).Apply(context.TODO(), sObj, am.ApplyOptions{
		FieldManager: "convox-system",
	})
	if err != nil {
		return "", err
	}
	return key, nil
}

func (p *Provider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	return nil, errors.WithStack(fmt.Errorf("unimplemented"))
}

func (p *Provider) SystemMetrics(opts structs.MetricsOptions) (structs.Metrics, error) {
	ms, err := p.MetricScraper.GetRackMetrics(opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return ms, nil
}

func (p *Provider) SystemProcesses(opts structs.SystemProcessesOptions) (structs.Processes, error) {
	ns := p.Namespace

	if common.DefaultBool(opts.All, false) {
		ns = ""
	}

	labelSelector := fmt.Sprintf("system=convox,rack=%s,service", p.Name)
	pds, err := p.ListPodsFromInformer(ns, labelSelector)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	pss := structs.Processes{}

	for _, pd := range pds.Items {
		ps, err := p.processFromPod(pd)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		pss = append(pss, *ps)
	}

	ms, err := p.MetricsClient.MetricsV1beta1().PodMetricses(ns).List(context.TODO(), am.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		p.logger.Errorf("failed to fetch pod metrics: %s", err)
	} else {

		metricsByPod := map[string]metricsv1beta1.PodMetrics{}
		for _, m := range ms.Items {
			metricsByPod[m.Name] = m
		}

		for i := range pss {
			if m, has := metricsByPod[pss[i].Id]; has && len(m.Containers) > 0 {
				pss[i].Cpu, pss[i].Memory = calculatePodCpuAndMem(&m)
			}
		}
	}

	sort.Slice(pss, pss.Less)

	return pss, nil
}

func (p *Provider) SystemReleases() (structs.Releases, error) {
	return nil, errors.WithStack(fmt.Errorf("release history is unavailable"))
}

func (p *Provider) SystemUninstall(name string, w io.Writer, opts structs.SystemUninstallOptions) error {
	return errors.WithStack(fmt.Errorf("direct rack doesn't support uninstall, make sure you are not using RACK_URL environment variable"))
}

func (p *Provider) SystemUpdate(opts structs.SystemUpdateOptions) error {
	return errors.WithStack(fmt.Errorf("direct rack doesn't support update, make sure you are not using RACK_URL environment variable"))
}

func (p *Provider) updateJwtSignKey() (string, error) {
	signKey := uuid.NewV4().String()
	sObj := &corev1.SecretApplyConfiguration{
		TypeMetaApplyConfiguration: amv1.TypeMetaApplyConfiguration{
			Kind:       options.String("Secret"),
			APIVersion: options.String("v1"),
		},
		ObjectMetaApplyConfiguration: &amv1.ObjectMetaApplyConfiguration{
			Name: options.String(ConvoxJwtSecretName),
			Labels: map[string]string{
				"system": "convox",
				"rack":   p.Name,
			},
		},
		Data: map[string][]byte{
			"signKey": []byte(signKey),
		},
	}
	_, err := p.Cluster.CoreV1().Secrets(p.Namespace).Apply(context.TODO(), sObj, am.ApplyOptions{
		FieldManager: "convox-system",
	})
	return base64.StdEncoding.EncodeToString([]byte(signKey)), err
}

func (p *Provider) createOrUpdateFlowSchema(namespace string, saNames []string) error {
	p.logger.Logf("Creating or updating flow schema for service accounts %s in namespace %s", strings.Join(saNames, ", "), namespace)

	fs := &flowcontrolv1.FlowSchema{
		ObjectMeta: metav1.ObjectMeta{
			Name: "convox-high-priority",
		},
		Spec: flowcontrolv1.FlowSchemaSpec{
			MatchingPrecedence: 1001,
			PriorityLevelConfiguration: flowcontrolv1.PriorityLevelConfigurationReference{
				Name: "workload-high",
			},
			DistinguisherMethod: &flowcontrolv1.FlowDistinguisherMethod{
				Type: flowcontrolv1.FlowDistinguisherMethodByUserType,
			},
			Rules: []flowcontrolv1.PolicyRulesWithSubjects{
				{
					Subjects: []flowcontrolv1.Subject{},
					ResourceRules: []flowcontrolv1.ResourcePolicyRule{
						{
							Verbs:        []string{"*"},
							APIGroups:    []string{"*"},
							Resources:    []string{"*"},
							ClusterScope: true,
						},
					},
					NonResourceRules: []flowcontrolv1.NonResourcePolicyRule{
						{
							Verbs:           []string{"*"},
							NonResourceURLs: []string{"*"},
						},
					},
				},
			},
		},
	}

	for _, saName := range saNames {
		fs.Spec.Rules[0].Subjects = append(fs.Spec.Rules[0].Subjects, flowcontrolv1.Subject{
			Kind: "ServiceAccount",
			ServiceAccount: &flowcontrolv1.ServiceAccountSubject{
				Name:      saName,
				Namespace: namespace,
			},
		})
	}

	flowSchemas := p.Cluster.FlowcontrolV1().FlowSchemas()
	ctx := context.Background()

	existing, err := flowSchemas.Get(ctx, fs.Name, metav1.GetOptions{})
	if err == nil && existing != nil {
		fs.ResourceVersion = existing.ResourceVersion
		_, err = flowSchemas.Update(ctx, fs, metav1.UpdateOptions{})
		return err
	}

	_, err = flowSchemas.Create(ctx, fs, metav1.CreateOptions{})
	return err
}
