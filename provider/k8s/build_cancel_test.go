package k8s

import (
	"context"
	"fmt"
	"testing"

	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func cancelBuildRecord(t *testing.T, kc *cvfake.Clientset, ns, id, status, process string) {
	t.Helper()
	_, err := kc.ConvoxV1().Builds(ns).Create(&ca.Build{
		ObjectMeta: am.ObjectMeta{
			Name:   id,
			Labels: map[string]string{"app": "app1"},
		},
		Spec: ca.BuildSpec{
			Ended:   "20200101.000000.000000000",
			Process: process,
			Started: "20200101.000000.000000000",
			Status:  status,
		},
	})
	require.NoError(t, err)
}

func cancelBuildPod(t *testing.T, kk *fake.Clientset, ns, name string) {
	t.Helper()
	_, err := kk.CoreV1().Pods(ns).Create(context.TODO(), &ac.Pod{
		ObjectMeta: am.ObjectMeta{Name: name, Namespace: ns, UID: types.UID(name + "-uid")},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func buildPodDeleteAttempted(kk *fake.Clientset) bool {
	for _, a := range kk.Actions() {
		if a.GetVerb() == "delete" && a.GetResource().Resource == "pods" {
			return true
		}
	}
	return false
}

func TestBuildCancelRunningBuild(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	cancelBuildRecord(t, kc, "rack1-app1", "build1", "running", "build-abcde")
	cancelBuildPod(t, kk, "rack1-app1", "build-abcde")
	podDeletePreconditions(t, kk)

	require.NoError(t, p.BuildCancel("app1", "BUILD1"))
	require.False(t, podStillExists(t, kk, "rack1-app1", "build-abcde"))

	kb, err := kc.ConvoxV1().Builds("rack1-app1").Get("build1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "failed", kb.Spec.Status)
	require.Contains(t, kb.Spec.Reason, "cancelled")
	require.NotEqual(t, "20200101.000000.000000000", kb.Spec.Ended)
}

func TestBuildCancelPodInBuildNamespace(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	p.PodSecurityStandard = "baseline"
	p.PodSecurityMode = "enforce"
	createAppNamespace(t, kk, "rack1", "app1")
	cancelBuildRecord(t, kc, "rack1-app1", "build1", "running", "build-abcde")
	cancelBuildPod(t, kk, "rack1-build-app1", "build-abcde")

	require.NoError(t, p.BuildCancel("app1", "build1"))
	require.False(t, podStillExists(t, kk, "rack1-build-app1", "build-abcde"))

	kb, err := kc.ConvoxV1().Builds("rack1-app1").Get("build1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "failed", kb.Spec.Status)
}

func TestBuildCancelPodAlreadyGone(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	cancelBuildRecord(t, kc, "rack1-app1", "build1", "running", "build-abcde")

	require.NoError(t, p.BuildCancel("app1", "build1"))

	kb, err := kc.ConvoxV1().Builds("rack1-app1").Get("build1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "failed", kb.Spec.Status)
}

func TestBuildCancelRejectsNonRunningBuild(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  string
		process string
	}{
		{"created", "created", ""},
		{"complete", "complete", "build-abcde"},
		{"failed", "failed", "build-abcde"},
		{"running without a process", "running", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, kk, kc := minimalProvider(t)
			createAppNamespace(t, kk, "rack1", "app1")
			cancelBuildRecord(t, kc, "rack1-app1", "build1", tc.status, tc.process)
			cancelBuildPod(t, kk, "rack1-app1", "build-abcde")

			err := p.BuildCancel("app1", "build1")
			require.Error(t, err)
			require.Contains(t, err.Error(), "build BUILD1 is not running")
			require.False(t, buildPodDeleteAttempted(kk))
			require.True(t, podStillExists(t, kk, "rack1-app1", "build-abcde"))

			kb, gerr := kc.ConvoxV1().Builds("rack1-app1").Get("build1", am.GetOptions{})
			require.NoError(t, gerr)
			require.Equal(t, tc.status, kb.Spec.Status)
		})
	}
}

// A build that finishes between the gate and the write keeps its own outcome:
// the pod is gone either way, but the record must not be stamped failed.
func TestBuildCancel_CompletedDuringCall_KeepsOwnOutcome(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	cancelBuildRecord(t, kc, "rack1-app1", "build1", "running", "build-abcde")
	cancelBuildPod(t, kk, "rack1-app1", "build-abcde")

	kk.PrependReactor("delete", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		kb, err := kc.ConvoxV1().Builds("rack1-app1").Get("build1", am.GetOptions{})
		require.NoError(t, err)
		kb.Spec.Release = "release1"
		kb.Spec.Status = "complete"
		_, err = kc.ConvoxV1().Builds("rack1-app1").Update(kb)
		require.NoError(t, err)
		return false, nil, nil
	})

	require.NoError(t, p.BuildCancel("app1", "build1"))
	require.False(t, podStillExists(t, kk, "rack1-app1", "build-abcde"))

	kb, err := kc.ConvoxV1().Builds("rack1-app1").Get("build1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "complete", kb.Spec.Status)
	require.Equal(t, "release1", kb.Spec.Release)
	require.Empty(t, kb.Spec.Reason)
}

// The pod outlives PSA enforcement being turned on: processBuildNamespace now
// answers with the build namespace while the pod is still in the app namespace.
func TestBuildCancelFallsBackToTheOtherNamespace(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	p.PodSecurityStandard = "baseline"
	p.PodSecurityMode = "enforce"
	createAppNamespace(t, kk, "rack1", "app1")
	cancelBuildRecord(t, kc, "rack1-app1", "build1", "running", "build-abcde")
	cancelBuildPod(t, kk, "rack1-app1", "build-abcde")

	require.NoError(t, p.BuildCancel("app1", "build1"))
	require.False(t, podStillExists(t, kk, "rack1-app1", "build-abcde"))
}

// A pod lookup that fails for any reason other than NotFound must not leave the
// user told the build was cancelled while its builder keeps running.
func TestBuildCancelDoesNotMarkTheBuildWhenTheLookupFails(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	cancelBuildRecord(t, kc, "rack1-app1", "build1", "running", "build-abcde")
	cancelBuildPod(t, kk, "rack1-app1", "build-abcde")

	kk.PrependReactor("get", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, ae.NewInternalError(fmt.Errorf("apiserver is unwell"))
	})

	require.Error(t, p.BuildCancel("app1", "build1"))
	require.False(t, buildPodDeleteAttempted(kk))

	kb, err := kc.ConvoxV1().Builds("rack1-app1").Get("build1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "running", kb.Spec.Status)
}

func TestBuildCancelReportsAConflictOnTheRecordWrite(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	cancelBuildRecord(t, kc, "rack1-app1", "build1", "running", "build-abcde")
	cancelBuildPod(t, kk, "rack1-app1", "build-abcde")

	kc.PrependReactor("update", "builds", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, ae.NewConflict(schema.GroupResource{Resource: "builds"}, "build1", fmt.Errorf("modified"))
	})

	err := p.BuildCancel("app1", "build1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "is being modified, try again")
}

func TestBuildCancelReportsAMissingBuild(t *testing.T) {
	p, kk, _ := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")

	err := p.BuildCancel("app1", "nosuch")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no such build: nosuch")
}
