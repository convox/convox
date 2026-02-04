package manifest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/convox/convox/pkg/options"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	autoscaling "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

const (
	PVCAccessModeReadWriteOnce = "ReadWriteOnce"
	PVCAccessModeReadOnlyMany  = "ReadOnlyMany"
	PVCAccessModeReadWriteMany = "ReadWriteMany"
)

type Service struct {
	Name string `yaml:"-"`

	Agent              ServiceAgent          `yaml:"agent,omitempty"`
	Annotations        Annotations           `yaml:"annotations,omitempty"`
	Build              ServiceBuild          `yaml:"build,omitempty"`
	Certificate        Certificate           `yaml:"certificate,omitempty"`
	Command            string                `yaml:"command,omitempty"`
	ConfigMounts       ConfigMounts          `yaml:"configMounts,omitempty"`
	Deployment         ServiceDeployment     `yaml:"deployment,omitempty"`
	DnsConfig          ServiceDnsConfig      `yaml:"dnsConfig,omitempty"`
	Domains            ServiceDomains        `yaml:"domain,omitempty"`
	Drain              int                   `yaml:"drain,omitempty"`
	DisableHostUsers   bool                  `yaml:"disableHostUsers,omitempty"`
	Environment        Environment           `yaml:"environment,omitempty"`
	GrpcHealthEnabled  bool                  `yaml:"grpcHealthEnabled,omitempty"`
	Health             ServiceHealth         `yaml:"health,omitempty"`
	Liveness           ServiceLiveness       `yaml:"liveness,omitempty"`
	StartupProbe       ServiceStartupProbe   `yaml:"startupProbe,omitempty"`
	Image              string                `yaml:"image,omitempty"`
	Init               bool                  `yaml:"init,omitempty"`
	InitContainer      *InitContainer        `yaml:"initContainer,omitempty"`
	Internal           bool                  `yaml:"internal,omitempty"`
	InternalRouter     bool                  `yaml:"internalRouter,omitempty"`
	IngressAnnotations Annotations           `yaml:"ingressAnnotations,omitempty"`
	Labels             Labels                `yaml:"labels,omitempty"`
	NodeAffinityLabels Affinities            `yaml:"nodeAffinityLabels,omitempty"`
	NodeSelectorLabels Labels                `yaml:"nodeSelectorLabels,omitempty"`
	Lifecycle          ServiceLifecycle      `yaml:"lifecycle,omitempty"`
	Port               ServicePortScheme     `yaml:"port,omitempty"`
	Ports              []ServicePortProtocol `yaml:"ports,omitempty"`
	Privileged         bool                  `yaml:"privileged,omitempty"`
	Resources          []string              `yaml:"resources,omitempty"`
	Scale              ServiceScale          `yaml:"scale,omitempty"`
	Singleton          bool                  `yaml:"singleton,omitempty"`
	Sticky             bool                  `yaml:"sticky,omitempty"`
	Termination        ServiceTermination    `yaml:"termination,omitempty"`
	Test               string                `yaml:"test,omitempty"`
	Timeout            int                   `yaml:"timeout,omitempty"`
	Tls                ServiceTls            `yaml:"tls,omitempty"`
	Volumes            []string              `yaml:"volumes,omitempty"`
	VolumeOptions      []VolumeOption        `yaml:"volumeOptions,omitempty"`
	Whitelist          string                `yaml:"whitelist,omitempty"`
	AccessControl      AccessControlOptions  `yaml:"accessControl,omitempty"`
}

type Affinities []Affinity

type Affinity struct {
	Label  string `yaml:"label"`
	Weight int    `yaml:"weight"`
	Value  string `yaml:"value"`
}

type Annotations []Annotation

type Annotation struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type InitContainer struct {
	Image         string         `yaml:"image,omitempty"`
	Command       string         `yaml:"command,omitempty"`
	VolumeOptions []VolumeOption `yaml:"volumeOptions,omitempty"`
	ConfigMounts  ConfigMounts   `yaml:"configMounts,omitempty"`
}

type VolumeOption struct {
	EmptyDir *VolumeEmptyDir `yaml:"emptyDir,omitempty"`
	AwsEfs   *VolumeAwsEfs   `yaml:"awsEfs,omitempty"`
}

func (v VolumeOption) Validate() error {
	if v.EmptyDir != nil {
		return v.EmptyDir.Validate()
	}
	if v.AwsEfs != nil {
		return v.AwsEfs.Validate()
	}
	return nil
}

type VolumeEmptyDir struct {
	Id        string `yaml:"id"`
	Medium    string `yaml:"medium,omitempty"`
	MountPath string `yaml:"mountPath"`
}

func (v VolumeEmptyDir) Validate() error {
	if v.Id == "" {
		return fmt.Errorf("emptyDir.id is required")
	}
	if v.MountPath == "" {
		return fmt.Errorf("emptyDir.mountPath is required")
	}
	if v.Medium != "" {
		if v.Medium != "Memory" {
			return fmt.Errorf("emptyDir.medium's allowed values: Memory")
		}
	}
	return nil
}

type VolumeAwsEfs struct {
	Id string `yaml:"id"`

	AccessMode   string `yaml:"accessMode,omitempty"`
	MountPath    string `yaml:"mountPath"`
	StorageClass string `yaml:"storageClass,omitempty"`
	VolumeHandle string `yaml:"volumeHandle,omitempty"`
}

func (v VolumeAwsEfs) Validate() error {
	if v.Id == "" {
		return fmt.Errorf("awsEfs.id is required")
	}
	if v.MountPath == "" {
		return fmt.Errorf("awsEfs.mountPath is required")
	}

	allowedModes := []string{
		PVCAccessModeReadOnlyMany,
		PVCAccessModeReadWriteMany,
		PVCAccessModeReadWriteOnce,
	}

	if !containsInStringSlice(allowedModes, v.AccessMode) {
		return fmt.Errorf("awsEfs.accessMode must be one of these values: %s", strings.Join(allowedModes, ", "))
	}
	return nil
}

func (v *VolumeAwsEfs) ProcessTemplate(efsFsId, app, service string) {
	if !strings.Contains(v.VolumeHandle, ":") {
		v.VolumeHandle = fmt.Sprintf("%s:%s", efsFsId, v.VolumeHandle)
	}

	v.VolumeHandle = strings.ReplaceAll(v.VolumeHandle, "[APP]", app)
	v.VolumeHandle = strings.ReplaceAll(v.VolumeHandle, "[SERVICE]", service)
}

type Services []Service

type Certificate struct {
	Id       string `yaml:"id,omitempty"`
	Duration string `yaml:"duration,omitempty"`
}

type ServiceAgent struct {
	Enabled bool `yaml:"enabled,omitempty"`
}

type ServiceAnnotations []string

type ServiceBuild struct {
	Args     []string `yaml:"args,omitempty"`
	Manifest string   `yaml:"manifest,omitempty"`
	Path     string   `yaml:"path,omitempty"`
}

type ServiceDeployment struct {
	Maximum int `yaml:"maximum,omitempty"`
	Minimum int `yaml:"minimum,omitempty"`
}

type ServiceDomains []string

type ServiceDnsConfig struct {
	Ndots int
}

type ServiceHealth struct {
	Disable  bool
	Grace    int
	Interval int
	Path     string
	Timeout  int
}

type ServiceLiveness struct {
	Grace            int    `yaml:"grace,omitempty"`
	Interval         int    `yaml:"interval,omitempty"`
	Path             string `yaml:"path,omitempty"`
	Timeout          int    `yaml:"timeout,omitempty"`
	SuccessThreshold int    `yaml:"successThreshold,omitempty"`
	FailureThreshold int    `yaml:"failureThreshold,omitempty"`
}

type ServiceStartupProbe struct {
	Grace            int    `yaml:"grace,omitempty"`
	Interval         int    `yaml:"interval,omitempty"`
	Path             string `yaml:"path,omitempty"`
	TcpSocketPort    string `yaml:"tcpSocketPort,omitempty"`
	Timeout          int    `yaml:"timeout,omitempty"`
	SuccessThreshold int    `yaml:"successThreshold,omitempty"`
	FailureThreshold int    `yaml:"failureThreshold,omitempty"`
}

type ServiceLifecycle struct {
	PreStop   string `yaml:"preStop,omitempty"`
	PostStart string `yaml:"postStart,omitempty"`
}

type ServicePortProtocol struct {
	Port     int    `yaml:"port,omitempty"`
	Protocol string `yaml:"protocol,omitempty"`
}

type ServicePortScheme struct {
	Port   int    `yaml:"port,omitempty"`
	Scheme string `yaml:"scheme,omitempty"`
}

type ServiceScale struct {
	Count   ServiceScaleCount
	Cpu     int
	Gpu     ServiceScaleGpu `yaml:"gpu,omitempty"`
	Memory  int
	Limit   ServiceResourceLimit `yaml:"limit,omitempty"`
	Targets ServiceScaleTargets  `yaml:"targets,omitempty"`
	Keda    *ServiceScaleKeda    `yaml:"keda,omitempty"`
	VPA     *VPA                 `yaml:"vpa,omitempty"`
}

func (ss ServiceScale) IsKedaEnabled() bool {
	return ss.Keda != nil && len(ss.Keda.Triggers) > 0
}

func (ss ServiceScale) IsVpaEnabled() bool {
	return ss.VPA != nil
}

type KedaScaledObjectParameters struct {
	MinCount    int32
	MaxCount    int32
	ServiceName string
	Namespace   string
}

func (ss Service) KedaScaledObject(params KedaScaledObjectParameters) *kedav1alpha1.ScaledObject {
	so := kedav1alpha1.ScaledObject{}
	if ss.Scale.Keda == nil {
		return &so
	}

	so.TypeMeta.Kind = "ScaledObject"
	so.TypeMeta.APIVersion = "keda.sh/v1alpha1"
	so.ObjectMeta.Name = params.ServiceName
	so.ObjectMeta.Namespace = params.Namespace

	so.Spec.ScaleTargetRef = &kedav1alpha1.ScaleTarget{
		Name: params.ServiceName,
		Kind: "Deployment",
	}
	so.Spec.Triggers = ss.Scale.Keda.Triggers
	so.Spec.MinReplicaCount = &params.MinCount
	so.Spec.MaxReplicaCount = &params.MaxCount
	so.Spec.CooldownPeriod = ss.Scale.Keda.CooldownPeriod
	so.Spec.PollingInterval = ss.Scale.Keda.PollingInterval
	so.Spec.Advanced = ss.Scale.Keda.Advanced
	so.Spec.Fallback = ss.Scale.Keda.Fallback
	so.Spec.IdleReplicaCount = ss.Scale.Keda.IdleReplicaCount

	for i := range so.Spec.Triggers {
		if so.Spec.Triggers[i].AuthenticationRef == nil {
			auth := ss.DefaultTriggerAuthentionIfAws(params.Namespace)
			so.Spec.Triggers[i].AuthenticationRef = &kedav1alpha1.AuthenticationRef{
				Name: auth.ObjectMeta.Name,
				Kind: auth.TypeMeta.Kind,
			}
		}
	}

	return &so
}

func (ss Service) DefaultTriggerAuthentionIfAws(namespace string) *kedav1alpha1.TriggerAuthentication {
	if os.Getenv("PROVIDER") == "aws" {
		auth := &kedav1alpha1.TriggerAuthentication{
			TypeMeta: v1.TypeMeta{
				Kind:       "TriggerAuthentication",
				APIVersion: "keda.sh/v1alpha1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "keda-aws-auth-default",
				Namespace: namespace,
			},
			Spec: kedav1alpha1.TriggerAuthenticationSpec{
				PodIdentity: &kedav1alpha1.AuthPodIdentity{
					Provider:      "aws",
					IdentityOwner: options.String("keda"),
				},
			},
		}
		return auth
	}
	return nil
}

type ServiceScaleKeda struct {
	// +optional
	PollingInterval *int32 `yaml:"pollingInterval,omitempty"`
	// +optional
	InitialCooldownPeriod *int32 `yaml:"initialCooldownPeriod,omitempty"`
	// +optional
	CooldownPeriod *int32 `yaml:"cooldownPeriod,omitempty"`
	// +optional
	IdleReplicaCount *int32 `yaml:"idleReplicaCount,omitempty"`
	// +optional
	Advanced *kedav1alpha1.AdvancedConfig `yaml:"advanced,omitempty"`

	Triggers []kedav1alpha1.ScaleTriggers `yaml:"triggers"`
	// +optional
	Fallback *kedav1alpha1.Fallback `yaml:"fallback,omitempty"`
}

type VPA struct {
	// default: Recreate
	// allowed: Off, Initial, Recreate, InPlaceOrRecreate
	UpdateMode        string  `yaml:"updateMode,omitempty"`
	MinCpu            *string `yaml:"minCpu,omitempty"`
	MaxCpu            *string `yaml:"maxCpu,omitempty"`
	MinMem            *string `yaml:"minMem,omitempty"`
	MaxMem            *string `yaml:"maxMem,omitempty"`
	CpuOnly           *bool   `yaml:"cpuOnly,omitempty"`
	MemOnly           *bool   `yaml:"memOnly,omitempty"`
	UpdateRequestOnly *bool   `yaml:"updateRequestOnly,omitempty"`
}

func (vpa *VPA) Validate() error {
	allowedModes := []string{"Off", "Initial", "Recreate"}
	match := false
	if vpa.UpdateMode == "" {
		return fmt.Errorf("vpa.updateMode is required. Allowed values: %s", strings.Join(allowedModes, ", "))
	}

	for _, mode := range allowedModes {
		if strings.ToLower(vpa.UpdateMode) == strings.ToLower(mode) {
			vpa.UpdateMode = mode
			match = true
		}
	}

	if !match {
		return fmt.Errorf("vpa.updateMode must be one of these values: %s", strings.Join(allowedModes, ", "))
	}

	fmt.Println(vpa.CpuOnly, vpa.MemOnly)

	if vpa.CpuOnly != nil && vpa.MemOnly != nil && *vpa.CpuOnly && *vpa.MemOnly {
		return fmt.Errorf("vpa.cpuOnly and vpa.memOnly cannot both be true")
	}
	return nil
}

func (vpa *VPA) VpaObject(serviceName, namespace string, labels map[string]string) (*vpav1.VerticalPodAutoscaler, error) {
	if vpa.Validate() != nil {
		return nil, vpa.Validate()
	}

	updateMode := vpav1.UpdateMode(vpa.UpdateMode)
	vpaObj := &vpav1.VerticalPodAutoscaler{
		TypeMeta: v1.TypeMeta{
			Kind:       "VerticalPodAutoscaler",
			APIVersion: "autoscaling.k8s.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: vpav1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscaling.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       serviceName,
				APIVersion: "apps/v1",
			},
			UpdatePolicy: &vpav1.PodUpdatePolicy{
				UpdateMode: &updateMode,
			},
		},
	}

	if vpa.MinCpu != nil || vpa.MinMem != nil || vpa.MaxCpu != nil || vpa.MaxMem != nil {
		vpaObj.Spec.ResourcePolicy = &vpav1.PodResourcePolicy{
			ContainerPolicies: []vpav1.ContainerResourcePolicy{
				{
					ContainerName: "*",
					MinAllowed:    corev1.ResourceList{},
					MaxAllowed:    corev1.ResourceList{},
				},
			},
		}

		if vpa.MinCpu != nil {
			qty, err := resource.ParseQuantity(fmt.Sprintf("%sm", *vpa.MinCpu))
			if err != nil {
				return nil, fmt.Errorf("invalid vpa.minCpu value: %v", err)
			}
			vpaObj.Spec.ResourcePolicy.ContainerPolicies[0].MinAllowed[corev1.ResourceCPU] = qty
		}
		if vpa.MinMem != nil {
			qty, err := resource.ParseQuantity(fmt.Sprintf("%sMi", *vpa.MinMem))
			if err != nil {
				return nil, fmt.Errorf("invalid vpa.minMem value: %v", err)
			}
			vpaObj.Spec.ResourcePolicy.ContainerPolicies[0].MinAllowed[corev1.ResourceMemory] = qty
		}
		if vpa.MaxCpu != nil {
			qty, err := resource.ParseQuantity(fmt.Sprintf("%sm", *vpa.MaxCpu))
			if err != nil {
				return nil, fmt.Errorf("invalid vpa.maxCpu value: %v", err)
			}
			vpaObj.Spec.ResourcePolicy.ContainerPolicies[0].MaxAllowed[corev1.ResourceCPU] = qty
		}
		if vpa.MaxMem != nil {
			qty, err := resource.ParseQuantity(fmt.Sprintf("%sMi", *vpa.MaxMem))
			if err != nil {
				return nil, fmt.Errorf("invalid vpa.maxMem value: %v", err)
			}
			vpaObj.Spec.ResourcePolicy.ContainerPolicies[0].MaxAllowed[corev1.ResourceMemory] = qty
		}
	}

	if vpa.CpuOnly != nil || vpa.MemOnly != nil || vpa.UpdateRequestOnly != nil {
		if vpaObj.Spec.ResourcePolicy == nil {
			vpaObj.Spec.ResourcePolicy = &vpav1.PodResourcePolicy{
				ContainerPolicies: []vpav1.ContainerResourcePolicy{
					{
						ContainerName: "*",
						MinAllowed:    corev1.ResourceList{},
						MaxAllowed:    corev1.ResourceList{},
					},
				},
			}
		}

		if vpa.CpuOnly != nil && *vpa.CpuOnly {
			vpaObj.Spec.ResourcePolicy.ContainerPolicies[0].ControlledResources = &[]corev1.ResourceName{corev1.ResourceCPU}
		}

		if vpa.MemOnly != nil && *vpa.MemOnly {
			vpaObj.Spec.ResourcePolicy.ContainerPolicies[0].ControlledResources = &[]corev1.ResourceName{corev1.ResourceMemory}
		}

		if vpa.UpdateRequestOnly != nil && *vpa.UpdateRequestOnly {
			v := vpav1.ContainerControlledValuesRequestsOnly
			vpaObj.Spec.ResourcePolicy.ContainerPolicies[0].ControlledValues = &v
		}
	}

	return vpaObj, nil
}

type ServiceResourceLimit struct {
	Cpu    int
	Memory int
}

type ServiceScaleCount struct {
	Min int
	Max int
}
type ServiceScaleExternalMetric struct {
	AverageValue *float64          `yaml:"averageValue,omitempty"`
	MatchLabels  map[string]string `yaml:"matchLabels,omitempty"`
	Name         string            `yaml:"name"`
	Value        *float64          `yaml:"value,omitempty"`
}

type ServiceScaleExternalMetrics []ServiceScaleExternalMetric

type ServiceScaleGpu struct {
	Count  int
	Vendor string
}

type ServiceScaleMetric struct {
	Aggregate  string
	Dimensions map[string]string
	Namespace  string
	Name       string
	Value      float64
}

type ServiceScaleMetrics []ServiceScaleMetric

type ServiceScaleTargets struct {
	Cpu      int
	Custom   ServiceScaleMetrics
	External ServiceScaleExternalMetrics
	Memory   int
	Requests int
}

type ServiceTermination struct {
	Grace int `yaml:"grace,omitempty"`
}

type ServiceTls struct {
	Redirect bool
}

// skipcq
func (s Service) BuildHash(key string) string {
	return fmt.Sprintf("%x", sha256.Sum224([]byte(fmt.Sprintf("key=%q build[path=%q, manifest=%q, args=%v] image=%q", key, s.Build.Path, s.Build.Manifest, s.Build.Args, s.Image))))
}

// skipcq
func (s Service) Domain() string {
	if len(s.Domains) < 1 {
		return ""
	}

	return s.Domains[0]
}

// skipcq
func (s Service) EnvironmentDefaults() map[string]string {
	defaults := map[string]string{}

	for _, e := range s.Environment {
		switch parts := strings.SplitN(e, "=", 2); len(parts) {
		case 2:
			defaults[parts[0]] = parts[1]
		}
	}

	return defaults
}

// skipcq
func (s Service) EnvironmentKeys() string {
	kh := map[string]bool{}

	for _, e := range s.Environment {
		kh[strings.SplitN(e, "=", 2)[0]] = true
	}

	keys := []string{}

	for k := range kh {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return strings.Join(keys, ",")
}

// skipcq
func (s Service) GetName() string {
	return s.Name
}

// skipcq
func (s Service) Autoscale() bool {
	if s.Agent.Enabled {
		return false
	}

	switch {
	case s.Scale.Count.Min == s.Scale.Count.Max:
		return false
	case s.Scale.Targets.Cpu > 0:
		return true
	case len(s.Scale.Targets.Custom) > 0:
		return true
	case s.Scale.Targets.Memory > 0:
		return true
	case s.Scale.Targets.Requests > 0:
		return true
	}

	return false
}

type ServiceResource struct {
	Name string
	Env  string
}

func (sr ServiceResource) GetConfigMapKey() string {
	parts := strings.Split(sr.Env, "_")
	key := parts[len(parts)-1]

	for _, en := range AdditionalEnvNames {
		if key == en {
			return key
		}
	}

	return DEFAULT_RESOURCE_ENV_NAME
}

func (s Service) AnnotationsMap() map[string]string {
	annotations := map[string]string{}

	for _, a := range s.Annotations {
		annotations[a.Key] = a.Value
	}

	return annotations
}

// skipcq
func (s Service) IngressAnnotationsMap() map[string]string {
	annotations := map[string]string{}

	for _, a := range s.Annotations {
		annotations[a.Key] = a.Value
	}

	return annotations
}

// skipcq
func (s Service) ResourceMap() []ServiceResource {
	srs := []ServiceResource{}

	for _, r := range s.Resources {
		parts := strings.SplitN(r, ":", 2)

		switch len(parts) {
		case 1:
			envs := Resource{Name: parts[0]}.LoadEnv()
			for _, e := range envs {
				srs = append(srs, ServiceResource{Name: parts[0], Env: e})
			}
		case 2:
			srs = append(srs, ServiceResource{Name: parts[0], Env: strings.TrimSpace(parts[1])})
		}
	}

	return srs
}

// skipcq
func (s Service) ResourcesName() []string {
	srs := []string{}

	for _, r := range s.Resources {
		parts := strings.SplitN(r, ":", 2)
		srs = append(srs, parts[0])
	}

	return srs
}

func (ss Services) External() Services {
	return ss.Filter(func(s Service) bool {
		return !s.Internal && !s.InternalRouter
	})
}

func (ss Services) Filter(fn func(s Service) bool) Services {
	fss := Services{}

	// skipcq
	for _, s := range ss {
		if fn(s) {
			fss = append(fss, s)
		}
	}

	return fss
}

func (ss Services) InternalRouter() Services {
	return ss.Filter(func(s Service) bool {
		return s.InternalRouter
	})
}

func (ss Services) Routable() Services {
	return ss.Filter(func(s Service) bool {
		return s.Port.Port > 0
	})
}

type AccessControlOptions struct {
	AWSPodIdentity *AWSPodIdentityOptions `yaml:"awsPodIdentity,omitempty"`
}

type AWSPodIdentityOptions struct {
	PolicyArns []string `yaml:"policyArns"`
}
