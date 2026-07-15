package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const manifestRunFlags = `services:
  web:
    build: .
    termination:
      grace: 300
  plain:
    build: .
  hooks:
    build: .
    lifecycle:
      postStart: /bin/poststart.sh arg
      preStop: /bin/prestop.sh
  onehook:
    build: .
    lifecycle:
      postStart: echo hi
  sel:
    build: .
    nodeSelectorLabels:
      team: ml
`

func runFlagsProvider(t *testing.T) (*Provider, *fake.Clientset) {
	t.Helper()
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestRunFlags)
	return p, kk
}

func runAndGetPod(t *testing.T, p *Provider, kk *fake.Clientset, app, service string, opts structs.ProcessRunOptions) *ac.Pod { //nolint:gocritic // hugeParam: mirrors runAndGetPodSpec and ProcessRun by-value opts
	t.Helper()
	ps, err := p.ProcessRun(app, service, opts)
	require.NoError(t, err)
	require.NotNil(t, ps)

	pod, err := kk.CoreV1().Pods(p.AppNamespace(app)).Get(context.TODO(), ps.Id, am.GetOptions{})
	require.NoError(t, err)
	return pod
}

func TestProcessRun_GraceCarryover(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "web", structs.ProcessRunOptions{Release: options.String("rel1")}).Spec
	require.NotNil(t, spec.TerminationGracePeriodSeconds)
	require.Equal(t, int64(300), *spec.TerminationGracePeriodSeconds)
}

func TestProcessRun_GraceDefaultThirty(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{Release: options.String("rel1")}).Spec
	require.NotNil(t, spec.TerminationGracePeriodSeconds)
	require.Equal(t, int64(30), *spec.TerminationGracePeriodSeconds)
}

func TestProcessRun_GraceOverrideBeatsManifest(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "web", structs.ProcessRunOptions{
		Release:          options.String("rel1"),
		TerminationGrace: options.Int(900),
	}).Spec
	require.NotNil(t, spec.TerminationGracePeriodSeconds)
	require.Equal(t, int64(900), *spec.TerminationGracePeriodSeconds)
}

func TestProcessRun_GraceZeroOverride(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "web", structs.ProcessRunOptions{
		Release:          options.String("rel1"),
		TerminationGrace: options.Int(0),
	}).Spec
	require.NotNil(t, spec.TerminationGracePeriodSeconds)
	require.Equal(t, int64(0), *spec.TerminationGracePeriodSeconds)
}

func TestProcessRun_GraceNegativeRejected(t *testing.T) {
	p, _ := runFlagsProvider(t)
	_, err := p.ProcessRun("app1", "web", structs.ProcessRunOptions{
		Release:          options.String("rel1"),
		TerminationGrace: options.Int(-1),
	})
	require.Error(t, err)
}

func TestProcessRun_BuildPodGraceNil(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "web", structs.ProcessRunOptions{
		Release: options.String("rel1"),
		IsBuild: true,
	}).Spec
	require.Nil(t, spec.TerminationGracePeriodSeconds, "build pods must keep the k8s default (nil), never be force-set to 0")
}

func TestProcessRun_Annotations(t *testing.T) {
	p, kk := runFlagsProvider(t)
	pod := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:     options.String("rel1"),
		Annotations: options.String("team=infra,karpenter.sh/do-not-disrupt=true"),
	})
	require.Equal(t, "infra", pod.Annotations["team"])
	require.Equal(t, "true", pod.Annotations["karpenter.sh/do-not-disrupt"])
}

func TestProcessRun_AnnotationsMalformedKeyRejected(t *testing.T) {
	p, _ := runFlagsProvider(t)
	_, err := p.ProcessRun("app1", "plain", structs.ProcessRunOptions{
		Release:     options.String("rel1"),
		Annotations: options.String("bad key=v"),
	})
	require.Error(t, err)
}

func TestProcessRun_AnnotationsNoEqualsRejected(t *testing.T) {
	p, _ := runFlagsProvider(t)
	_, err := p.ProcessRun("app1", "plain", structs.ProcessRunOptions{
		Release:     options.String("rel1"),
		Annotations: options.String("novalue"),
	})
	require.Error(t, err)
}

func TestProcessRun_AnnotationsEmptyNoOp(t *testing.T) {
	p, kk := runFlagsProvider(t)
	pod := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:     options.String("rel1"),
		Annotations: options.String(""),
	})
	_, hasTeam := pod.Annotations["team"]
	require.False(t, hasTeam)
}

func TestProcessRun_AnnotationsTrailingComma(t *testing.T) {
	p, kk := runFlagsProvider(t)
	pod := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:     options.String("rel1"),
		Annotations: options.String("a=b,"),
	})
	require.Equal(t, "b", pod.Annotations["a"])
}

func TestProcessRun_AnnotationValueNotLabelValidated(t *testing.T) {
	p, kk := runFlagsProvider(t)
	pod := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:     options.String("rel1"),
		Annotations: options.String("team=infra/prod team"),
	})
	require.Equal(t, "infra/prod team", pod.Annotations["team"])
}

func TestProcessRun_Labels(t *testing.T) {
	p, kk := runFlagsProvider(t)
	pod := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release: options.String("rel1"),
		Labels:  options.String("purpose=batch"),
	})
	require.Equal(t, "batch", pod.Labels["purpose"])
	require.Equal(t, "app1", pod.Labels["app"])
}

func TestProcessRun_LabelsReservedRejected(t *testing.T) {
	p, _ := runFlagsProvider(t)
	for _, k := range []string{"app", "rack", "service", "system", "type", "name", "release", "service-type"} {
		_, err := p.ProcessRun("app1", "plain", structs.ProcessRunOptions{
			Release: options.String("rel1"),
			Labels:  options.String(k + "=evil"),
		})
		require.Error(t, err, "reserved label %q must be rejected", k)
	}
}

func TestProcessRun_LabelsInvalidValueRejected(t *testing.T) {
	p, _ := runFlagsProvider(t)
	_, err := p.ProcessRun("app1", "plain", structs.ProcessRunOptions{
		Release: options.String("rel1"),
		Labels:  options.String("purpose=has space"),
	})
	require.Error(t, err)
}

func TestProcessRun_LifecycleBothHooks(t *testing.T) {
	p, kk := runFlagsProvider(t)
	c := runAndGetPod(t, p, kk, "app1", "hooks", structs.ProcessRunOptions{
		Release:             options.String("rel1"),
		UseServiceLifecycle: options.Bool(true),
	}).Spec.Containers[0]
	require.NotNil(t, c.Lifecycle)
	require.NotNil(t, c.Lifecycle.PostStart)
	require.Equal(t, []string{"/bin/poststart.sh", "arg"}, c.Lifecycle.PostStart.Exec.Command)
	require.NotNil(t, c.Lifecycle.PreStop)
	require.Equal(t, []string{"/bin/prestop.sh"}, c.Lifecycle.PreStop.Exec.Command)
}

func TestProcessRun_LifecycleOneHook(t *testing.T) {
	p, kk := runFlagsProvider(t)
	c := runAndGetPod(t, p, kk, "app1", "onehook", structs.ProcessRunOptions{
		Release:             options.String("rel1"),
		UseServiceLifecycle: options.Bool(true),
	}).Spec.Containers[0]
	require.NotNil(t, c.Lifecycle)
	require.NotNil(t, c.Lifecycle.PostStart)
	require.Nil(t, c.Lifecycle.PreStop)
}

func TestProcessRun_LifecycleNoHooksNoOp(t *testing.T) {
	p, kk := runFlagsProvider(t)
	c := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:             options.String("rel1"),
		UseServiceLifecycle: options.Bool(true),
	}).Spec.Containers[0]
	require.Nil(t, c.Lifecycle)
}

func TestProcessRun_LifecycleWithoutReleasePinsCurrent(t *testing.T) {
	p, kk := runFlagsProvider(t)
	c := runAndGetPod(t, p, kk, "app1", "hooks", structs.ProcessRunOptions{
		UseServiceLifecycle: options.Bool(true),
	}).Spec.Containers[0]
	require.NotNil(t, c.Lifecycle)
	require.NotNil(t, c.Lifecycle.PostStart)
}

func TestProcessRun_NodeAffinity(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:      options.String("rel1"),
		NodeAffinity: options.String("gpu=v:80"),
	}).Spec
	require.NotNil(t, spec.Affinity)
	require.NotNil(t, spec.Affinity.NodeAffinity)
	pref := spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	require.Len(t, pref, 1)
	require.Equal(t, int32(80), pref[0].Weight)
	require.Equal(t, "gpu", pref[0].Preference.MatchExpressions[0].Key)
	require.Equal(t, ac.NodeSelectorOpIn, pref[0].Preference.MatchExpressions[0].Operator)
	require.Equal(t, []string{"v"}, pref[0].Preference.MatchExpressions[0].Values)
}

func TestProcessRun_NodeAffinityDefaultWeight(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:      options.String("rel1"),
		NodeAffinity: options.String("gpu=v"),
	}).Spec
	pref := spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	require.Len(t, pref, 1)
	require.Equal(t, int32(100), pref[0].Weight)
}

func TestProcessRun_NodeAffinitySameKeyTwoTerms(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:      options.String("rel1"),
		NodeAffinity: options.String("zone=east:80,zone=west:50"),
	}).Spec
	pref := spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	require.Len(t, pref, 2)
}

func TestProcessRun_NodeAffinityRejects(t *testing.T) {
	p, _ := runFlagsProvider(t)
	for _, bad := range []string{"gpu:80", "gpu=", "gpu=v:0", "gpu=v:101", "gpu=v:abc", "gpu=v:80:90", "bad key=v"} {
		_, err := p.ProcessRun("app1", "plain", structs.ProcessRunOptions{
			Release:      options.String("rel1"),
			NodeAffinity: options.String(bad),
		})
		require.Error(t, err, "node-affinity %q must be rejected", bad)
	}
}

func TestProcessRun_NodeAffinityAppendsToInheritedRequired(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "sel", structs.ProcessRunOptions{
		Release:      options.String("rel1"),
		NodeAffinity: options.String("gpu=v:80"),
	}).Spec
	require.NotNil(t, spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
	require.Len(t, spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, 1)
}

func TestProcessRun_Tolerations(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:     options.String("rel1"),
		Tolerations: options.String("k1=v:NoSchedule,k2:NoExecute,k3"),
	}).Spec
	require.Len(t, spec.Tolerations, 3)

	require.Equal(t, "k1", spec.Tolerations[0].Key)
	require.Equal(t, ac.TolerationOpEqual, spec.Tolerations[0].Operator)
	require.Equal(t, "v", spec.Tolerations[0].Value)
	require.Equal(t, ac.TaintEffectNoSchedule, spec.Tolerations[0].Effect)

	require.Equal(t, "k2", spec.Tolerations[1].Key)
	require.Equal(t, ac.TolerationOpExists, spec.Tolerations[1].Operator)
	require.Equal(t, "", spec.Tolerations[1].Value)
	require.Equal(t, ac.TaintEffectNoExecute, spec.Tolerations[1].Effect)

	require.Equal(t, "k3", spec.Tolerations[2].Key)
	require.Equal(t, ac.TolerationOpExists, spec.Tolerations[2].Operator)
	require.Equal(t, ac.TaintEffect(""), spec.Tolerations[2].Effect)
}

func TestProcessRun_TolerationsMultiSameKey(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:     options.String("rel1"),
		Tolerations: options.String("k:NoSchedule,k:NoExecute"),
	}).Spec
	require.Len(t, spec.Tolerations, 2)
	require.Equal(t, ac.TaintEffectNoSchedule, spec.Tolerations[0].Effect)
	require.Equal(t, ac.TaintEffectNoExecute, spec.Tolerations[1].Effect)
}

func TestProcessRun_TolerationsEmptySegmentNoWildcard(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "plain", structs.ProcessRunOptions{
		Release:     options.String("rel1"),
		Tolerations: options.String("k:NoSchedule,"),
	}).Spec
	require.Len(t, spec.Tolerations, 1)
	require.Equal(t, "k", spec.Tolerations[0].Key)
}

func TestProcessRun_TolerationsRejects(t *testing.T) {
	p, _ := runFlagsProvider(t)
	for _, bad := range []string{"k=:NoSchedule", "=v:NoSchedule", "k:BadEffect", "bad key:NoSchedule"} {
		_, err := p.ProcessRun("app1", "plain", structs.ProcessRunOptions{
			Release:     options.String("rel1"),
			Tolerations: options.String(bad),
		})
		require.Error(t, err, "toleration %q must be rejected", bad)
	}
}

func TestProcessRun_OrderingWithNodeLabels(t *testing.T) {
	p, kk := runFlagsProvider(t)
	spec := runAndGetPod(t, p, kk, "app1", "sel", structs.ProcessRunOptions{
		Release:      options.String("rel1"),
		NodeLabels:   options.String("custom-pool=debug"),
		NodeAffinity: options.String("gpu=v:80"),
		Tolerations:  options.String("t=1:NoSchedule"),
	}).Spec

	require.Equal(t, map[string]string{"custom-pool": "debug"}, spec.NodeSelector)

	require.NotNil(t, spec.Affinity, "node-affinity must survive the --node-labels reset")
	require.Len(t, spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, 1)

	var found bool
	for _, tol := range spec.Tolerations {
		if tol.Key == "t" {
			found = true
		}
	}
	require.True(t, found, "user toleration must survive the --node-labels reset")
}
