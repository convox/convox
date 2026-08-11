package k8s

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// the fake clientset ignores delete preconditions, so enforce them the way the
// api server does: a uid mismatch is a conflict
func podDeletePreconditions(t *testing.T, c *fake.Clientset) {
	t.Helper()
	c.PrependReactor("delete", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		d, ok := action.(k8stesting.DeleteActionImpl)
		if !ok {
			t.Fatalf("delete action was %T, preconditions are not being emulated", action)
		}
		if d.DeleteOptions.Preconditions == nil || d.DeleteOptions.Preconditions.UID == nil {
			return false, nil, nil
		}
		if o, err := c.Tracker().Get(action.GetResource(), action.GetNamespace(), d.Name); err == nil {
			if pod, ok := o.(*ac.Pod); ok && pod.UID != *d.DeleteOptions.Preconditions.UID {
				return true, nil, kerr.NewConflict(action.GetResource().GroupResource(), d.Name, fmt.Errorf("uid in precondition does not match"))
			}
		}
		return false, nil, nil
	})
}

func podFixture(name string, uid types.UID) *ac.Pod {
	return &ac.Pod{ObjectMeta: am.ObjectMeta{Name: name, Namespace: "ns1", UID: uid}}
}

func TestCleanupPodDeletesOnlyTheObservedPod(t *testing.T) {
	tests := []struct {
		name      string
		live      *ac.Pod
		observed  *ac.Pod
		deleteErr error
		wantExist bool
		wantErr   bool
	}{
		{
			name:     "deletes the pod it observed",
			live:     podFixture("build-abc", "uid-1"),
			observed: podFixture("build-abc", "uid-1"),
		},
		{
			name:      "leaves a replacement that reused the name",
			live:      podFixture("db-1", "uid-2"),
			observed:  podFixture("db-1", "uid-1"),
			wantExist: true,
		},
		{
			name:     "tolerates a pod that is already gone",
			observed: podFixture("db-1", "uid-1"),
		},
		{
			name:      "surfaces an unexpected delete failure",
			live:      podFixture("db-1", "uid-1"),
			observed:  podFixture("db-1", "uid-1"),
			deleteErr: kerr.NewForbidden(ac.Resource("pods"), "db-1", fmt.Errorf("denied")),
			wantExist: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			kk := fake.NewSimpleClientset()
			podDeletePreconditions(t, kk)
			if tt.deleteErr != nil {
				kk.PrependReactor("delete", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, tt.deleteErr
				})
			}
			if tt.live != nil {
				_, err := kk.CoreV1().Pods("ns1").Create(context.TODO(), tt.live, am.CreateOptions{})
				require.NoError(t, err)
			}

			c := &PodController{Provider: &Provider{Cluster: kk}}
			err := c.cleanupPod(tt.observed)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			_, err = kk.CoreV1().Pods("ns1").Get(context.TODO(), tt.observed.Name, am.GetOptions{})
			if tt.wantExist {
				require.NoError(t, err)
				return
			}
			require.True(t, kerr.IsNotFound(err))
		})
	}
}
