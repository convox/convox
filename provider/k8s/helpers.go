package k8s

import (
	"bytes"
	"context"
	"crypto/sha256"
	gojson "encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/scheme"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	tc "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

const (
	ScannerStartSize = 4096
	ScannerMaxSize   = 1024 * 1024
)

var (
	kubernetesNameFilter = regexp.MustCompile(`[^a-z-.]`)
)

type Patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// skipcq
func (p *Provider) environment(a *structs.App, r *structs.Release, s manifest.Service, e structs.Environment) (map[string]string, error) {
	env := map[string]string{}

	if p.ContextTID() != "" {
		env["TID"] = p.ContextTID()
	} else {
		for k, v := range p.systemEnvironment() {
			env[k] = v
		}
	}

	for k, v := range p.appEnvironment(a) {
		env[k] = v
	}

	for k, v := range p.releaseEnvironment(a, r) {
		env[k] = v
	}

	if r.Build != "" {
		b, err := p.BuildGet(a.Name, r.Build)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		for k, v := range p.buildEnvironment(a, b) {
			env[k] = v
		}
	}

	for k, v := range p.serviceEnvironment(a, s) {
		env[k] = v
	}

	for k, v := range s.EnvironmentDefaults() {
		env[k] = v
	}

	for k, v := range e {
		env[k] = v
	}

	return env, nil
}

func (*Provider) appEnvironment(a *structs.App) map[string]string {
	return map[string]string{
		"APP": a.Name,
	}
}

// skipcq
func (*Provider) buildEnvironment(a *structs.App, b *structs.Build) map[string]string {
	return map[string]string{
		"BUILD":             b.Id,
		"BUILD_DESCRIPTION": b.Description,
		"BUILD_GIT_SHA":     b.GitSha,
	}
}

// skipcq
func (*Provider) releaseEnvironment(a *structs.App, r *structs.Release) map[string]string {
	return map[string]string{
		"RELEASE": r.Id,
	}
}

// skipcq
func (p *Provider) serviceEnvironment(a *structs.App, s manifest.Service) map[string]string {
	env := map[string]string{
		"SERVICE": s.Name,
	}

	if s.Port.Port > 0 {
		env["PORT"] = strconv.Itoa(s.Port.Port)
	}

	return env
}

func (p *Provider) systemEnvironment() map[string]string {
	if p.FeatureGates[options.FeatureGateSystemEnvDisable] {
		return map[string]string{}
	}
	return map[string]string{
		"RACK":     p.Name,
		"RACK_URL": fmt.Sprintf("https://convox:%s@api.%s.svc.cluster.local:5443", p.Password, p.Namespace),
	}
}

func (*Provider) volumeFrom(app, service, v string) string {
	from := strings.Split(v, ":")[0]

	switch {
	case systemVolume(from):
		return from
	case strings.Contains(v, ":"):
		return path.Join("/mnt/volumes", app, "app", from)
	default:
		return path.Join("/mnt/volumes", app, "service", service, from)
	}
}

func (p *Provider) volumeName(app, v string) string {
	hash := sha256.Sum256([]byte(v))
	name := fmt.Sprintf("%s-%s-%x", p.Name, app, hash[0:20])
	if len(name) > 63 {
		name = name[0:62]
	}
	return name
}

func (p *Provider) volumeSources(app, service string, vs []string) []string {
	vsh := map[string]bool{}

	for _, v := range vs {
		vsh[p.volumeFrom(app, service, v)] = true
	}

	var vsu []string

	for v := range vsh {
		vsu = append(vsu, v)
	}

	sort.Strings(vsu)

	return vsu
}

func (p *Provider) parseTidFromNamespace(ns string) string {
	ns = strings.TrimPrefix(ns, p.Name+"-")
	parts := strings.SplitN(ns, "-", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func (p *Provider) parseAppFromNamespace(ns string) string {
	ns = strings.TrimPrefix(ns, p.Name+"-")
	parts := strings.SplitN(ns, "-", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ns
}

func nameFilter(name string) string {
	return kubernetesNameFilter.ReplaceAllString(name, "")
}

func nameFilterV2(name string) string {
	name = strings.ToLower(name)
	var builder strings.Builder
	for _, char := range name {
		if unicode.IsLetter(char) || (unicode.IsDigit(char) && builder.Len() > 0) || char == '-' {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func primaryContainer(cs []ac.Container, app string) (*ac.Container, error) {
	if len(cs) != 1 {
		return nil, fmt.Errorf("no containers found")
	}

	switch cs[0].Name {
	case app, "main":
	default:
		return nil, fmt.Errorf("unexpected container name")
	}

	return &(cs[0]), nil
}

func systemVolume(v string) bool {
	switch v {
	case "/cgroup/":
		return true
	case "/proc/":
		return true
	case "/sys/fs/cgroup/":
		return true
	case "/sys/kernel/debug/":
		return true
	case "/var/log/audit/":
		return true
	case "/var/run/":
		return true
	case "/var/run/docker.sock":
		return true
	case "/var/snap/microk8s/current/docker.sock":
		return true
	}
	return false
}

func volumeTo(v string) (string, error) {
	switch parts := strings.SplitN(v, ":", 2); len(parts) {
	case 1:
		return parts[0], nil
	case 2:
		return parts[1], nil
	default:
		return "", errors.WithStack(fmt.Errorf("invalid volume %q", v))
	}
}

func toCpuCore(millicore int64) float64 {
	// 1000m (milicores) = 1 core = 1 vCPU = 1 AWS vCPU = 1 GCP Core
	return float64(millicore) / 1000
}

func toMemMB(bytes int64) float64 {
	// 1024*1024 bytes = 1 MiB
	return float64(bytes) / (1024 * 1024)
}

func calculatePodCpuAndMem(m *metricsv1beta1.PodMetrics) (cpu float64, mem float64) {
	if m == nil {
		return 0, 0
	}

	var cpuTotal, memTotal int64
	for _, c := range m.Containers {
		cpuTotal += c.Usage.Cpu().MilliValue()
		memTotal += c.Usage.Memory().Value()
	}
	return toCpuCore(cpuTotal), toMemMB(memTotal)
}

func aggregateMetricByPeriod(m *structs.Metric, period int64) *structs.Metric {
	sort.Slice(m.Values, func(i, j int) bool {
		return m.Values[i].Time.After(m.Values[j].Time)
	})

	vs := structs.MetricValues{}
	for _, v := range m.Values {
		withinPeriod := len(vs) > 0 && vs[len(vs)-1].Time.Sub(v.Time).Seconds() <= float64(period)
		if withinPeriod {
			newv := vs[len(vs)-1]
			newv.Count++
			newv.Maximum = math.Max(newv.Maximum, v.Maximum)
			newv.Minimum = math.Min(newv.Minimum, v.Minimum)
			newv.Sum += newv.Sum
		} else {
			vs = append(vs, v)
		}
	}

	for i := range vs {
		if vs[i].Count > 0 {
			vs[i].Average = vs[i].Sum / vs[i].Count
		}
	}

	return &structs.Metric{
		Name:   m.Name,
		Values: vs,
	}
}

func filterMetricByStart(m *structs.Metric, start time.Time) *structs.Metric {
	vs := structs.MetricValues{}
	for _, v := range m.Values {
		if v.Time.After(start) {
			vs = append(vs, v)
		}
	}

	return &structs.Metric{
		Name:   m.Name,
		Values: vs,
	}
}

func caculatePercentage(cur, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return (cur / total) * 100
}

func RunUsingLeaderElection(ns, name string, cluster kubernetes.Interface, onStarted func(context.Context), onStopped func()) error {
	identifier, err := os.Hostname()
	if err != nil {
		return err
	}

	fmt.Printf("starting elector: %s/%s (%s)\n", ns, name, identifier)

	eb := record.NewBroadcaster()
	eb.StartRecordingToSink(&tc.EventSinkImpl{Interface: cluster.CoreV1().Events("")})

	recorder := eb.NewRecorder(scheme.Scheme, ac.EventSource{Component: name})

	rl := &resourcelock.LeaseLock{
		LeaseMeta: am.ObjectMeta{Namespace: ns, Name: name},
		Client:    cluster.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      identifier,
			EventRecorder: recorder,
		},
	}

	el, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 60 * time.Second,
		RenewDeadline: 15 * time.Second,
		RetryPeriod:   5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: onStarted,
			OnStoppedLeading: onStopped,
		},
	})
	if err != nil {
		return err
	}

	ctx, _ := context.WithCancel(context.Background())

	go el.Run(ctx)

	return nil
}

func SerializeK8sObjToYaml(obj runtime.Object) ([]byte, error) {
	serializer := json.NewYAMLSerializer(json.DefaultMetaFactory, nil, nil)

	var buf []byte
	w := bytes.NewBuffer(buf)
	err := serializer.Encode(obj, w)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

type tempStateLogStorage struct {
	lock      sync.Mutex
	s         map[string][]string
	threshold int
}

func (t *tempStateLogStorage) Add(tid, key, value string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	tkey := tid + key
	if _, has := t.s[tkey]; !has {
		t.s[tkey] = []string{}
	}
	t.s[tkey] = append(t.s[tkey], value)

	l := len(t.s[tkey])
	if l > t.threshold {
		t.s[tkey] = t.s[tkey][l-t.threshold:]
	}
}

func (t *tempStateLogStorage) Get(tid, key string) []string {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.s[tid+key]
}

func (t *tempStateLogStorage) Reset(tid, key string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.s[tid+key] = []string{}
}

type resourceSubstitution struct {
	App     string `json:"app"`
	RType   string `json:"type"`
	RName   string `json:"resource"`
	StateId string `json:"stateId"`
	Tid     string `json:"tid"`
}

func resourceSubstitutionId(r *resourceSubstitution) string {
	jsonData, _ := gojson.Marshal(r)
	mapData := map[string]string{}
	_ = gojson.Unmarshal(jsonData, &mapData)

	subsId := "##|"
	for k, v := range mapData {
		subsId = subsId + fmt.Sprintf("%s:%s|", k, v)
	}
	subsId += "##"
	return subsId
}

func parseResourceSubstitutionId(id string) *resourceSubstitution {
	kv := make(map[string]string)
	parts := strings.Split(id, "|")
	for _, p := range parts {
		if p == "" || p == "##" { // skip empty and delimiters
			continue
		}
		if idx := strings.Index(p, ":"); idx > 0 {
			k := p[:idx]
			v := p[idx+1:]
			kv[k] = v
		}
	}

	rs := &resourceSubstitution{}
	jsonData, _ := gojson.Marshal(kv)
	_ = gojson.Unmarshal(jsonData, rs)

	return rs
}

func patchBytes(patch map[string]interface{}) ([]byte, error) {
	return gojson.Marshal(patch)
}
