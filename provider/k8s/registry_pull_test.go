package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	tmock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	ac "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testRegistry = "registry.rack1.example"

type registryPullTestEngine struct {
	mock.TestEngine
}

func (*registryPullTestEngine) RegistryPullCredentials() (string, string, string) {
	return testRegistry, "docker", "secret1"
}

func registryPullProvider(cs *fake.Clientset) *Provider {
	return &Provider{
		Cluster:   cs,
		Engine:    &registryPullTestEngine{},
		Name:      "rack1",
		Namespace: "rack1-system",
		logger:    logger.Discard,
	}
}

func pullSecrets(names ...string) []ac.LocalObjectReference {
	refs := []ac.LocalObjectReference{}
	for _, n := range names {
		refs = append(refs, ac.LocalObjectReference{Name: n})
	}
	return refs
}

func pullSecretNames(refs []ac.LocalObjectReference) []string {
	names := []string{}
	for _, r := range refs {
		names = append(names, r.Name)
	}
	return names
}

func podTemplate(secrets []ac.LocalObjectReference, image string) ac.PodTemplateSpec {
	return ac.PodTemplateSpec{Spec: ac.PodSpec{
		ImagePullSecrets: secrets,
		Containers:       []ac.Container{{Name: "main", Image: image}},
	}}
}

func stuckPod(name, image, reason string, secrets []ac.LocalObjectReference) *ac.Pod {
	return &ac.Pod{
		ObjectMeta: am.ObjectMeta{Name: name, Namespace: "rack1-app1"},
		Spec: ac.PodSpec{
			ImagePullSecrets: secrets,
			Containers:       []ac.Container{{Name: "main", Image: image}},
		},
		Status: ac.PodStatus{ContainerStatuses: []ac.ContainerStatus{{
			Name:  "main",
			State: ac.ContainerState{Waiting: &ac.ContainerStateWaiting{Reason: reason}},
		}}},
	}
}

func registryPullFixtures() []runtime.Object {
	appImage := testRegistry + "/app1:web.B1"
	hub := pullSecrets("docker-hub-authentication")

	resource := &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{Name: "resource-database", Namespace: "rack1-app1"},
		Spec: appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Template: podTemplate(hub, "postgres:16-trixie"),
		},
	}
	resource.Spec.Template.Spec.InitContainers = []ac.Container{{Name: "init", Image: "docker.io/library/postgres:16"}}

	db := &appsv1.StatefulSet{
		ObjectMeta: am.ObjectMeta{Name: "db", Namespace: "rack1-app1"},
		Spec:       appsv1.StatefulSetSpec{Template: podTemplate(hub, "docker.io/library/busybox:1.36")},
	}
	db.Spec.Template.Spec.InitContainers = []ac.Container{{Name: "init", Image: appImage}}

	terminating := stuckPod("web-terminating", appImage, "ImagePullBackOff", nil)
	now := am.Now()
	terminating.DeletionTimestamp = &now
	terminating.Finalizers = []string{"test"}

	initStuck := stuckPod("web-init", "docker.io/library/busybox:1.36", "", nil)
	initStuck.Spec.InitContainers = []ac.Container{{Name: "init", Image: appImage}}
	initStuck.Status.ContainerStatuses = nil
	initStuck.Status.InitContainerStatuses = []ac.ContainerStatus{{
		Name:  "init",
		State: ac.ContainerState{Waiting: &ac.ContainerStateWaiting{Reason: "ErrImagePull"}},
	}}

	return []runtime.Object{
		newTenantNamespace("rack1-app1", "rack1", ""),
		&ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack1-system", Labels: map[string]string{"system": "convox", "rack": "rack1", "type": "rack"}}},
		&ac.ServiceAccount{ObjectMeta: am.ObjectMeta{Name: "default", Namespace: "rack1-app1"}, ImagePullSecrets: pullSecrets("other")},
		&ac.ServiceAccount{ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"}},
		&ac.ServiceAccount{ObjectMeta: am.ObjectMeta{Name: "api", Namespace: "rack1-system"}},
		&appsv1.Deployment{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
			Spec:       appsv1.DeploymentSpec{Template: podTemplate(hub, appImage)},
		},
		&appsv1.Deployment{
			ObjectMeta: am.ObjectMeta{Name: "worker", Namespace: "rack1-app1"},
			Spec:       appsv1.DeploymentSpec{Template: podTemplate(nil, appImage)},
		},
		resource,
		db,
		&appsv1.DaemonSet{
			ObjectMeta: am.ObjectMeta{Name: "agent", Namespace: "rack1-app1"},
			Spec:       appsv1.DaemonSetSpec{Template: podTemplate(hub, appImage)},
		},
		&batchv1.CronJob{
			ObjectMeta: am.ObjectMeta{Name: "timer-nightly", Namespace: "rack1-app1"},
			Spec: batchv1.CronJobSpec{JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{
				Template: podTemplate(hub, appImage),
			}}},
		},
		&appsv1.Deployment{
			ObjectMeta: am.ObjectMeta{Name: "api", Namespace: "rack1-system"},
			Spec:       appsv1.DeploymentSpec{Template: podTemplate(hub, appImage)},
		},
		stuckPod("web-stuck", appImage, "ImagePullBackOff", nil),
		stuckPod("hub-stuck", "docker.io/library/busybox:1.36", "ErrImagePull", nil),
		stuckPod("web-has-secret", appImage, "ImagePullBackOff", pullSecrets(registryPullSecretName)),
		stuckPod("web-running", appImage, "", nil),
		terminating,
		initStuck,
	}
}

func TestReconcileRegistryPull(t *testing.T) {
	cs := fake.NewSimpleClientset(registryPullFixtures()...)
	p := registryPullProvider(cs)
	ctx := context.Background()

	require.NoError(t, p.reconcileRegistryPull(ctx))

	s, err := cs.CoreV1().Secrets("rack1-app1").Get(ctx, registryPullSecretName, am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, ac.SecretTypeDockerConfigJson, s.Type)
	require.Equal(t, map[string]string{"system": "convox", "rack": "rack1"}, s.Labels)

	var cfg struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	require.NoError(t, json.Unmarshal(s.Data[ac.DockerConfigJsonKey], &cfg))
	require.Equal(t, base64.StdEncoding.EncodeToString([]byte("docker:secret1")), cfg.Auths[testRegistry].Auth)

	_, err = cs.CoreV1().Secrets("rack1-system").Get(ctx, registryPullSecretName, am.GetOptions{})
	require.True(t, k8serrors.IsNotFound(err))

	sa, err := cs.CoreV1().ServiceAccounts("rack1-app1").Get(ctx, "default", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"other", registryPullSecretName}, pullSecretNames(sa.ImagePullSecrets))

	sa, err = cs.CoreV1().ServiceAccounts("rack1-app1").Get(ctx, "web", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{registryPullSecretName}, pullSecretNames(sa.ImagePullSecrets))

	sa, err = cs.CoreV1().ServiceAccounts("rack1-system").Get(ctx, "api", am.GetOptions{})
	require.NoError(t, err)
	require.Empty(t, sa.ImagePullSecrets)

	patched := []string{"docker-hub-authentication", registryPullSecretName}

	d, err := cs.AppsV1().Deployments("rack1-app1").Get(ctx, "web", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, patched, pullSecretNames(d.Spec.Template.Spec.ImagePullSecrets))

	d, err = cs.AppsV1().Deployments("rack1-app1").Get(ctx, "worker", am.GetOptions{})
	require.NoError(t, err)
	require.Empty(t, d.Spec.Template.Spec.ImagePullSecrets)

	d, err = cs.AppsV1().Deployments("rack1-app1").Get(ctx, "resource-database", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"docker-hub-authentication"}, pullSecretNames(d.Spec.Template.Spec.ImagePullSecrets))

	d, err = cs.AppsV1().Deployments("rack1-system").Get(ctx, "api", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"docker-hub-authentication"}, pullSecretNames(d.Spec.Template.Spec.ImagePullSecrets))

	sts, err := cs.AppsV1().StatefulSets("rack1-app1").Get(ctx, "db", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, patched, pullSecretNames(sts.Spec.Template.Spec.ImagePullSecrets))

	ds, err := cs.AppsV1().DaemonSets("rack1-app1").Get(ctx, "agent", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, patched, pullSecretNames(ds.Spec.Template.Spec.ImagePullSecrets))

	cj, err := cs.BatchV1().CronJobs("rack1-app1").Get(ctx, "timer-nightly", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, patched, pullSecretNames(cj.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets))

	for name, deleted := range map[string]bool{
		"web-stuck":       true,
		"web-init":        true,
		"hub-stuck":       false,
		"web-has-secret":  false,
		"web-running":     false,
		"web-terminating": false,
	} {
		_, err := cs.CoreV1().Pods("rack1-app1").Get(ctx, name, am.GetOptions{})
		require.Equal(t, deleted, k8serrors.IsNotFound(err), name)
	}

	cs.ClearActions()
	require.NoError(t, p.reconcileRegistryPull(ctx))
	for _, a := range cs.Actions() {
		require.Contains(t, []string{"get", "list"}, a.GetVerb(), "second pass wrote %s %s", a.GetVerb(), a.GetResource().Resource)
	}
}

func TestReconcileRegistryPullNotOptedIn(t *testing.T) {
	cs := fake.NewSimpleClientset(registryPullFixtures()...)
	p := registryPullProvider(cs)
	p.Engine = &mock.TestEngine{}

	require.NoError(t, p.reconcileRegistryPull(context.Background()))
	require.Empty(t, cs.Actions())
}

func TestEnsureRegistryPullSecretAlreadyExists(t *testing.T) {
	cs := fake.NewSimpleClientset(newTenantNamespace("rack1-app1", "rack1", ""))
	p := registryPullProvider(cs)
	ctx := context.Background()

	require.NoError(t, p.ensureRegistryPullSecret(ctx, "rack1-app1"))
	require.NoError(t, p.ensureRegistryPullSecret(ctx, "rack1-app1"))

	cs.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewNotFound(ac.Resource("secrets"), registryPullSecretName)
	})
	require.NoError(t, p.ensureRegistryPullSecret(ctx, "rack1-app1"))
}

func TestProcessRunRegistryPullSecret(t *testing.T) {
	for name, tc := range map[string]struct {
		engine  Engine
		isBuild bool
		want    bool
	}{
		"opted in":     {engine: &registryPullTestEngine{}, want: true},
		"build":        {engine: &registryPullTestEngine{}, isBuild: true},
		"not opted in": {engine: &mock.TestEngine{}},
	} {
		t.Run(name, func(t *testing.T) {
			p, kk, kc := minimalProvider(t)
			p.Engine = tc.engine
			createAppNamespace(t, kk, "rack1", "app1")
			createBuild(t, kc, "rack1-app1", "build1")
			createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

			spec := runAndGetPodSpec(t, p, kk, "app1", "web", structs.ProcessRunOptions{
				Release: options.String("rel1"),
				IsBuild: tc.isBuild,
			})

			require.Equal(t, tc.want, slices.Contains(pullSecretNames(spec.ImagePullSecrets), registryPullSecretName))

			_, err := kk.CoreV1().Secrets("rack1-app1").Get(context.Background(), registryPullSecretName, am.GetOptions{})
			require.Equal(t, tc.want, err == nil)
		})
	}
}

func TestReleasePromoteRegistryPullSecret(t *testing.T) {
	for name, tc := range map[string]struct {
		engine Engine
		want   bool
	}{
		"opted in":     {engine: &registryPullTestEngine{}, want: true},
		"not opted in": {engine: &mock.TestEngine{}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("TEST", "true")

			p, kk, kc := minimalProvider(t)
			p.Engine = tc.engine

			_, err := kk.CoreV1().Namespaces().Create(context.Background(), &ac.Namespace{ObjectMeta: am.ObjectMeta{Name: p.Namespace}}, am.CreateOptions{})
			require.NoError(t, err)
			require.NoError(t, p.Initialize(structs.ProviderOptions{}))

			createAppNamespace(t, kk, "rack1", "app1")
			t.Cleanup(SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond))
			setAppStatus(t, kk, "rack1-app1", "Running")
			createBuild(t, kc, "rack1-app1", "build1")
			createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

			var rendered string
			aa, ok := p.Atom.(*atom.MockInterface)
			require.True(t, ok)
			aa.On("Apply", "rack1-app1", "app", tmock.Anything).Return(nil).Run(func(args tmock.Arguments) {
				cfg, ok := args.Get(2).(*atom.ApplyConfig)
				require.True(t, ok)
				rendered = string(cfg.Template)
			})

			require.NoError(t, p.ReleasePromote("app1", "rel1", structs.ReleasePromoteOptions{}))
			require.Equal(t, tc.want, strings.Contains(rendered, "- name: "+registryPullSecretName))

			_, err = kk.CoreV1().Secrets("rack1-app1").Get(context.Background(), registryPullSecretName, am.GetOptions{})
			require.Equal(t, tc.want, err == nil)
		})
	}
}
