package k8s

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type buildParamsEngine struct {
	*mock.TestEngine
}

func (buildParamsEngine) AppParameters() map[string]string {
	return map[string]string{"BuildArch": "", "BuildCpu": "", "BuildLabels": "", "BuildMem": ""}
}

func nativeBuildProvider(t *testing.T, params string) (*Provider, *fake.Clientset) {
	t.Helper()

	interval := buildHelperPollInterval
	buildHelperPollInterval = time.Millisecond
	t.Cleanup(func() { buildHelperPollInterval = interval })

	p, kk, _ := minimalProvider(t)
	p.ctx = context.Background()
	p.Engine = buildParamsEngine{&mock.TestEngine{}}
	p.BuildNodeEnabled = "true"
	p.BuildkitEnabled = "true"
	createAppNamespace(t, kk, "rack1", "app1")

	if params != "" {
		patch := fmt.Sprintf(`{"metadata":{"annotations":{"convox.com/params":%q}}}`, params)
		_, err := kk.CoreV1().Namespaces().Patch(context.TODO(), "rack1-app1", types.MergePatchType, []byte(patch), am.PatchOptions{})
		require.NoError(t, err)
	}

	kk.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		create, ok := action.(k8stesting.CreateAction)
		if !ok {
			return false, nil, nil
		}
		if pd, ok := create.GetObject().(*ac.Pod); ok && pd.Name == "" {
			pd.Name = pd.GenerateName + "abcde"
			pd.UID = "build-pod-uid"
		}
		return false, nil, nil
	})

	return p, kk
}

func bindBuildPod(t *testing.T, kk *fake.Clientset, ns string) {
	t.Helper()

	pd, err := kk.CoreV1().Pods(ns).Get(context.TODO(), "build-abcde", am.GetOptions{})
	require.NoError(t, err)
	pd.Spec.NodeName = "node1"
	_, err = kk.CoreV1().Pods(ns).Update(context.TODO(), pd, am.UpdateOptions{})
	require.NoError(t, err)
}

func waitHelperPod(t *testing.T, kk *fake.Clientset, ns, name string) {
	t.Helper()

	require.Eventually(t, func() bool {
		_, err := kk.CoreV1().Pods(ns).Get(context.TODO(), name, am.GetOptions{})
		return err == nil
	}, 5*time.Second, time.Millisecond)
}

func podEnv(pd *ac.Pod) map[string]string {
	env := map[string]string{}
	for _, e := range pd.Spec.Containers[0].Env {
		env[e.Name] = e.Value
	}
	return env
}

func podArch(pd *ac.Pod) string {
	if pd.Spec.Affinity == nil {
		return ""
	}
	for _, e := range pd.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions {
		if e.Key == "kubernetes.io/arch" {
			return strings.Join(e.Values, ",")
		}
	}
	return ""
}

func TestBuildCreateNativeArch(t *testing.T) {
	tests := []struct {
		name        string
		native      string
		archs       string
		params      string
		development bool
		buildNode   string
		pin         string
		helper      string
	}{
		{name: "off", archs: "amd64,arm64", buildNode: "true"},
		{name: "one arch", native: "amd64", archs: "arm64", buildNode: "true", pin: "arm64"},
		{name: "two archs", native: "amd64", archs: "amd64,arm64", buildNode: "true", pin: "amd64", helper: "arm64"},
		{name: "two archs arm home", native: "arm64", archs: "amd64,arm64", buildNode: "true", pin: "arm64", helper: "amd64"},
		{name: "no archs", native: "amd64", buildNode: "true"},
		{name: "BuildArch", native: "amd64", archs: "amd64,arm64", params: `{"BuildArch":"arm64"}`, buildNode: "true", pin: "arm64"},
		{name: "BuildLabels", native: "amd64", archs: "amd64,arm64", params: `{"BuildLabels":"convox.io/label=arm"}`, buildNode: "true"},
		{name: "development", native: "amd64", archs: "amd64,arm64", development: true, buildNode: "true", pin: "amd64"},
		{name: "no build node", native: "amd64", archs: "amd64,arm64"},
		{name: "unknown arch", native: "riscv64", archs: "amd64,arm64", buildNode: "true"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("BUILD_NATIVE_ARCH", test.native)
			t.Setenv("BUILD_ARCHS", test.archs)

			p, kk := nativeBuildProvider(t, test.params)
			p.BuildNodeEnabled = test.buildNode

			b, err := p.BuildCreate("app1", "object://app1/object.tgz", structs.BuildCreateOptions{Development: options.Bool(test.development)})
			require.NoError(t, err)
			require.Equal(t, "running", b.Status)

			if test.helper != "" {
				bindBuildPod(t, kk, "rack1-app1")
				waitHelperPod(t, kk, "rack1-app1", fmt.Sprintf("build-%s-%s", strings.ToLower(b.Id), test.helper))
			}

			pods, err := kk.CoreV1().Pods("rack1-app1").List(context.TODO(), am.ListOptions{})
			require.NoError(t, err)

			var build *ac.Pod
			helpers := []string{}
			for i := range pods.Items {
				if pods.Items[i].Labels["type"] == "build-helper" {
					helpers = append(helpers, podArch(&pods.Items[i]))
				} else {
					build = &pods.Items[i]
				}
			}
			require.NotNil(t, build)
			require.Equal(t, test.pin, podArch(build))

			env := podEnv(build)
			secrets, err := kk.CoreV1().Secrets("rack1-app1").List(context.TODO(), am.ListOptions{})
			require.NoError(t, err)
			services, err := kk.CoreV1().Services("rack1-app1").List(context.TODO(), am.ListOptions{})
			require.NoError(t, err)

			if test.helper == "" {
				require.Empty(t, helpers)
				require.Empty(t, secrets.Items)
				require.Empty(t, services.Items)
				for k := range env {
					require.False(t, strings.HasPrefix(k, "BUILD_HELPER_"), k)
				}
				return
			}

			require.Equal(t, []string{test.helper}, helpers)
			require.Len(t, secrets.Items, 1)
			require.Len(t, services.Items, 1)
			require.Equal(t, test.helper, env["BUILD_HELPER_ARCH"])
			require.Equal(t, test.archs, env["BUILD_ARCHS"])
		})
	}
}

func TestBuildCreateHelperObjects(t *testing.T) {
	t.Setenv("BUILD_NATIVE_ARCH", "amd64")
	t.Setenv("BUILD_ARCHS", "amd64,arm64")
	t.Setenv("BUILDKIT_HOST_PATH_CACHE_ENABLE", "true")
	t.Setenv("PROVIDER", "aws")

	p, kk := nativeBuildProvider(t, `{"BuildCpu":"2000","BuildMem":"4096"}`)
	p.Image = "convox/convox:test"

	b, err := p.BuildCreate("app1", "object://app1/object.tgz", structs.BuildCreateOptions{})
	require.NoError(t, err)

	id := strings.ToLower(b.Id)
	name := fmt.Sprintf("build-%s-arm64", id)
	host := fmt.Sprintf("%s.rack1-app1.svc.cluster.local", name)
	owner := []am.OwnerReference{{APIVersion: "v1", Kind: "Pod", Name: "build-abcde", UID: "build-pod-uid"}}
	labels := map[string]string{"system": "convox", "rack": "rack1", "app": "app1", "service-type": "build", "type": "build-helper", "build": id}

	bindBuildPod(t, kk, "rack1-app1")
	waitHelperPod(t, kk, "rack1-app1", name)

	build, err := kk.CoreV1().Pods("rack1-app1").Get(context.TODO(), "build-abcde", am.GetOptions{})
	require.NoError(t, err)
	env := podEnv(build)
	require.Equal(t, fmt.Sprintf("tcp://%s:1234", host), env["BUILD_HELPER_ADDR"])

	secret, err := kk.CoreV1().Secrets("rack1-app1").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	require.Empty(t, secret.Type)
	require.Equal(t, labels, secret.Labels)
	require.Equal(t, owner, secret.OwnerReferences)
	require.Len(t, secret.Data, 3)
	require.Equal(t, env["BUILD_HELPER_CA"], string(secret.Data["ca.crt"]))

	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(secret.Data["ca.crt"]))

	_, err = tls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"])
	require.NoError(t, err)
	server := parseCert(t, secret.Data["tls.crt"])
	_, err = server.Verify(x509.VerifyOptions{Roots: roots, DNSName: host, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
	require.NoError(t, err)
	_, err = server.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	require.Error(t, err)

	_, err = tls.X509KeyPair([]byte(env["BUILD_HELPER_CERT"]), []byte(env["BUILD_HELPER_KEY"]))
	require.NoError(t, err)
	client := parseCert(t, []byte(env["BUILD_HELPER_CERT"]))
	_, err = client.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	require.NoError(t, err)
	require.NotEqual(t, env["BUILD_HELPER_KEY"], string(secret.Data["tls.key"]))

	svc, err := kk.CoreV1().Services("rack1-app1").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, owner, svc.OwnerReferences)
	require.Equal(t, map[string]string{"type": "build-helper", "build": id}, svc.Spec.Selector)
	require.Equal(t, []ac.ServicePort{{Port: 1234, TargetPort: intstr.FromInt(1234)}}, svc.Spec.Ports)

	pd, err := kk.CoreV1().Pods("rack1-app1").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, labels, pd.Labels)
	require.Equal(t, owner, pd.OwnerReferences)
	require.Equal(t, "arm64", podArch(pd))
	require.Equal(t, []ac.Toleration{{Key: "dedicated", Operator: ac.TolerationOpExists, Effect: ac.TaintEffectNoSchedule}}, pd.Spec.Tolerations)
	require.Equal(t, ac.RestartPolicyNever, pd.Spec.RestartPolicy)
	require.Equal(t, int64(21600), *pd.Spec.ActiveDeadlineSeconds)

	c := pd.Spec.Containers[0]
	require.Equal(t, build.Spec.Containers[0].Image, c.Image)
	require.Equal(t, []string{"buildkitd", "--addr", "tcp://0.0.0.0:1234", "--tlscacert", "/certs/ca.crt", "--tlscert", "/certs/tls.crt", "--tlskey", "/certs/tls.key"}, c.Command)
	require.Empty(t, c.Args)
	require.Empty(t, c.Env)
	require.True(t, *c.SecurityContext.Privileged)
	require.Equal(t, "unconfined", pd.Annotations["container.apparmor.security.beta.kubernetes.io/app1"])
	require.True(t, resource.MustParse("2000m").Equal(c.Resources.Requests["cpu"]))
	require.True(t, resource.MustParse("4096Mi").Equal(c.Resources.Requests["memory"]))
	require.Equal(t, build.Spec.Containers[0].Resources.Requests, c.Resources.Requests)

	volumes := map[string]ac.Volume{}
	for _, v := range pd.Spec.Volumes {
		volumes[v.Name] = v
	}
	require.Equal(t, name, volumes["certs"].Secret.SecretName)
	require.NotContains(t, volumes, "buildkit-cache")
}

func parseCert(t *testing.T, data []byte) *x509.Certificate {
	t.Helper()

	block, _ := pem.Decode(data)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	return cert
}

func TestBuildCreateHelperBuildNamespace(t *testing.T) {
	t.Setenv("BUILD_NATIVE_ARCH", "amd64")
	t.Setenv("BUILD_ARCHS", "amd64,arm64")

	p, kk := nativeBuildProvider(t, "")
	p.PodSecurityStandard = "restricted"
	p.PodSecurityMode = "enforce"

	b, err := p.BuildCreate("app1", "object://app1/object.tgz", structs.BuildCreateOptions{})
	require.NoError(t, err)

	name := fmt.Sprintf("build-%s-arm64", strings.ToLower(b.Id))

	bindBuildPod(t, kk, "rack1-build-app1")
	waitHelperPod(t, kk, "rack1-build-app1", name)

	build, err := kk.CoreV1().Pods("rack1-build-app1").Get(context.TODO(), "build-abcde", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("tcp://%s.rack1-build-app1.svc.cluster.local:1234", name), podEnv(build)["BUILD_HELPER_ADDR"])

	_, err = kk.CoreV1().Pods("rack1-build-app1").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	_, err = kk.CoreV1().Services("rack1-build-app1").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	_, err = kk.CoreV1().Secrets("rack1-build-app1").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
}

func TestBuildCreateHelperWaitsForNode(t *testing.T) {
	t.Setenv("BUILD_NATIVE_ARCH", "amd64")
	t.Setenv("BUILD_ARCHS", "amd64,arm64")

	p, kk := nativeBuildProvider(t, "")

	b, err := p.BuildCreate("app1", "object://app1/object.tgz", structs.BuildCreateOptions{})
	require.NoError(t, err)
	require.Equal(t, "running", b.Status)
	require.Equal(t, "build-abcde", b.Process)

	name := fmt.Sprintf("build-%s-arm64", strings.ToLower(b.Id))

	require.Never(t, func() bool {
		_, perr := kk.CoreV1().Pods("rack1-app1").Get(context.TODO(), name, am.GetOptions{})
		_, serr := kk.CoreV1().Secrets("rack1-app1").Get(context.TODO(), name, am.GetOptions{})
		_, verr := kk.CoreV1().Services("rack1-app1").Get(context.TODO(), name, am.GetOptions{})
		return perr == nil || serr == nil || verr == nil
	}, 50*time.Millisecond, time.Millisecond)

	bindBuildPod(t, kk, "rack1-app1")
	waitHelperPod(t, kk, "rack1-app1", name)
}

func TestBuildHelperStart(t *testing.T) {
	bound := func(pd *ac.Pod) { pd.Spec.NodeName = "node1" }

	tests := []struct {
		name    string
		pod     func(*ac.Pod)
		getFail bool
		err     bool
		helper  bool
	}{
		{name: "no build pod"},
		{name: "deleting", pod: func(pd *ac.Pod) { bound(pd); pd.DeletionTimestamp = &am.Time{Time: time.Now()} }},
		{name: "succeeded", pod: func(pd *ac.Pod) { bound(pd); pd.Status.Phase = ac.PodSucceeded }},
		{name: "failed", pod: func(pd *ac.Pod) { bound(pd); pd.Status.Phase = ac.PodFailed }},
		{name: "no node", pod: func(*ac.Pod) {}, err: true},
		{name: "get error then bound", pod: bound, getFail: true, helper: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.err {
				timeout := buildHelperPollTimeout
				buildHelperPollTimeout = 20 * time.Millisecond
				t.Cleanup(func() { buildHelperPollTimeout = timeout })
			}

			p, kk := nativeBuildProvider(t, "")

			if test.pod != nil {
				pd := &ac.Pod{ObjectMeta: am.ObjectMeta{Name: "build-abcde", Namespace: "rack1-app1"}}
				test.pod(pd)
				_, err := kk.CoreV1().Pods("rack1-app1").Create(context.TODO(), pd, am.CreateOptions{})
				require.NoError(t, err)
			}

			if test.getFail {
				failed := false
				kk.PrependReactor("get", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
					if failed {
						return false, nil, nil
					}
					failed = true
					return true, nil, fmt.Errorf("timeout")
				})
			}

			h, err := p.buildHelperPrepare("app1", "BABCDEFGHIJ", "arm64")
			require.NoError(t, err)

			err = p.buildHelperStart("app1", "build-abcde", h, &structs.ProcessRunOptions{IsBuild: true, Image: options.String("convox/convox:test")})
			if test.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			_, err = kk.CoreV1().Pods("rack1-app1").Get(context.TODO(), h.name, am.GetOptions{})
			require.Equal(t, test.helper, err == nil)

			secrets, err := kk.CoreV1().Secrets("rack1-app1").List(context.TODO(), am.ListOptions{})
			require.NoError(t, err)
			if test.helper {
				require.Len(t, secrets.Items, 1)
			} else {
				require.Empty(t, secrets.Items)
			}
		})
	}
}

func TestBuildHelperStartFailureKeepsBuild(t *testing.T) {
	p, kk := nativeBuildProvider(t, "")
	kk.PrependReactor("create", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("quota")
	})

	_, err := kk.CoreV1().Pods("rack1-app1").Create(context.TODO(), &ac.Pod{
		ObjectMeta: am.ObjectMeta{Name: "build-abcde", Namespace: "rack1-app1"},
		Spec:       ac.PodSpec{NodeName: "node1"},
	}, am.CreateOptions{})
	require.NoError(t, err)

	h, err := p.buildHelperPrepare("app1", "BABCDEFGHIJ", "arm64")
	require.NoError(t, err)

	require.Error(t, p.buildHelperStart("app1", "build-abcde", h, &structs.ProcessRunOptions{IsBuild: true, Image: options.String("convox/convox:test")}))

	pods, err := kk.CoreV1().Pods("rack1-app1").List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Len(t, pods.Items, 1)
	require.Equal(t, "build-abcde", pods.Items[0].Name)

	services, err := kk.CoreV1().Services("rack1-app1").List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, services.Items)
}

func TestBuildHelperCreateWithoutBuildPod(t *testing.T) {
	p, kk := nativeBuildProvider(t, "")

	h, err := p.buildHelperPrepare("app1", "BABCDEFGHIJ", "arm64")
	require.NoError(t, err)

	require.Error(t, p.buildHelperCreate("app1", "build-gone", h, &structs.ProcessRunOptions{IsBuild: true}))

	secrets, err := kk.CoreV1().Secrets("rack1-app1").List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, secrets.Items)
}

func TestBuildCreateHelperTenant(t *testing.T) {
	t.Setenv("BUILD_NATIVE_ARCH", "amd64")
	t.Setenv("BUILD_ARCHS", "amd64,arm64")

	base, kk := nativeBuildProvider(t, "")
	base.FeatureGates = map[string]bool{options.FeatureGateTid: true}

	_, err := kk.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name:        "rack1-t1-app1",
			Annotations: map[string]string{"convox.com/lock": "false"},
			Labels:      map[string]string{"app": "app1", "name": "app1", "rack": "rack1", "system": "convox", "tid": "t1", "type": "app"},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	aa, ok := base.Atom.(*atom.MockInterface)
	require.True(t, ok)
	aa.On("Status", "rack1-t1-app1", "app").Return("Running", "rel1", nil)

	p, ok := base.WithContext(context.WithValue(context.Background(), structs.ConvoxTIDCtxKey, "t1")).(*Provider)
	require.True(t, ok)

	b, err := p.BuildCreate("app1", "object://app1/object.tgz", structs.BuildCreateOptions{})
	require.NoError(t, err)

	const ns = "rack1-build-t1-app1"
	name := fmt.Sprintf("build-%s-arm64", strings.ToLower(b.Id))

	bindBuildPod(t, kk, ns)
	waitHelperPod(t, kk, ns, name)

	build, err := kk.CoreV1().Pods(ns).Get(context.TODO(), "build-abcde", am.GetOptions{})
	require.NoError(t, err)

	env := podEnv(build)
	require.Equal(t, fmt.Sprintf("tcp://%s.%s.svc.cluster.local:1234", name, ns), env["BUILD_HELPER_ADDR"])
	_, err = tls.X509KeyPair([]byte(env["BUILD_HELPER_CERT"]), []byte(env["BUILD_HELPER_KEY"]))
	require.NoError(t, err)

	_, err = kk.CoreV1().Pods(ns).Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	_, err = kk.CoreV1().Services(ns).Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	_, err = kk.CoreV1().Secrets(ns).Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
}

func TestProcessRun_BuildNodeAffinity(t *testing.T) {
	build := ac.NodeSelectorRequirement{Key: "convox-build", Operator: ac.NodeSelectorOpIn, Values: []string{"true"}}
	arm := ac.NodeSelectorRequirement{Key: "kubernetes.io/arch", Operator: ac.NodeSelectorOpIn, Values: []string{"arm64"}}
	dedicated := ac.Toleration{Key: "dedicated", Operator: ac.TolerationOpExists, Effect: ac.TaintEffectNoSchedule}
	dedicatedNode := ac.Toleration{Key: "dedicated-node", Operator: ac.TolerationOpExists, Effect: ac.TaintEffectNoSchedule}

	required := func(exprs ...ac.NodeSelectorRequirement) *ac.Affinity {
		return &ac.Affinity{NodeAffinity: &ac.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &ac.NodeSelector{
				NodeSelectorTerms: []ac.NodeSelectorTerm{{MatchExpressions: exprs}},
			},
		}}
	}

	for name, tc := range map[string]struct {
		buildNode   string
		opts        structs.ProcessRunOptions
		affinity    *ac.Affinity
		tolerations []ac.Toleration
		selector    map[string]string
	}{
		"NoBuildNode":          {"", structs.ProcessRunOptions{IsBuild: true, BuildArch: options.String("arm64")}, nil, []ac.Toleration{dedicated}, nil},
		"BuildNode":            {"true", structs.ProcessRunOptions{IsBuild: true}, required(build), []ac.Toleration{dedicated}, nil},
		"EmptyArch":            {"true", structs.ProcessRunOptions{IsBuild: true, BuildArch: options.String("")}, required(build), []ac.Toleration{dedicated}, nil},
		"Arch":                 {"true", structs.ProcessRunOptions{IsBuild: true, BuildArch: options.String("arm64")}, required(build, arm), []ac.Toleration{dedicated}, nil},
		"BuildLabels":          {"true", structs.ProcessRunOptions{IsBuild: true, NodeLabels: options.String("convox.io/label=arm")}, required(build), []ac.Toleration{dedicatedNode, dedicated}, map[string]string{"convox.io/label": "arm"}},
		"NodeAffinityReplaced": {"true", structs.ProcessRunOptions{IsBuild: true, NodeAffinity: options.String("zone=a:10")}, required(build), []ac.Toleration{dedicated}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			p, kk, kc := minimalProvider(t)
			p.BuildNodeEnabled = tc.buildNode
			createAppNamespace(t, kk, "rack1", "app1")
			createBuild(t, kc, "rack1-app1", "build1")
			createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

			tc.opts.Release = options.String("rel1")
			tc.opts.Command = options.String("build")

			spec := runAndGetPodSpec(t, p, kk, "app1", "build", tc.opts)

			require.Equal(t, tc.affinity, spec.Affinity)
			require.Equal(t, tc.tolerations, spec.Tolerations)
			require.Equal(t, tc.selector, spec.NodeSelector)
		})
	}
}
