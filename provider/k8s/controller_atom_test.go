package k8s

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	atomv1 "github.com/convox/convox/pkg/atom/pkg/apis/atom/v1"
	afake "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned/fake"
	"github.com/convox/convox/pkg/kctl"
	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

func testAtom(status, version string) *atomv1.Atom {
	return &atomv1.Atom{
		ObjectMeta: am.ObjectMeta{Name: "app", Namespace: "rack1-app1"},
		Spec:       atomv1.AtomSpec{CurrentVersion: version, Dependencies: []string{"dep"}},
		Status:     atomv1.AtomStatus(status),
	}
}

func testAtomController(fa *afake.Clientset) *AtomController {
	return &AtomController{
		atom:                fa,
		dependencyProcessor: &sync.Map{},
		logger:              logger.New("ns=test"),
		provider:            &Provider{ctx: context.Background()},
	}
}

func TestAtomClearDependencies(t *testing.T) {
	tests := []struct {
		Name      string
		Live      *atomv1.Atom
		Conflicts int
		Want      []string
	}{
		{Name: "same version", Live: testAtom("Pending", "app-1"), Want: nil},
		{Name: "newer promote", Live: testAtom("Pending", "app-2"), Want: []string{"dep"}},
		{Name: "cancelled", Live: testAtom("Cancelled", "app-1"), Want: []string{"dep"}},
		{Name: "conflict", Live: testAtom("Pending", "app-1"), Conflicts: 1, Want: nil},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fa := afake.NewSimpleClientset(test.Live)

			conflicts := test.Conflicts
			fa.PrependReactor("update", "atoms", func(k8stesting.Action) (bool, runtime.Object, error) {
				if conflicts > 0 {
					conflicts--
					return true, nil, ae.NewConflict(schema.GroupResource{Resource: "atoms"}, "app", errors.New("modified"))
				}
				return false, nil, nil
			})

			require.NoError(t, testAtomController(fa).clearDependencies(testAtom("Pending", "app-1")))

			a, err := fa.AtomV1().Atoms("rack1-app1").Get(context.Background(), "app", am.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, test.Want, a.Spec.Dependencies)
			require.Zero(t, conflicts)
		})
	}
}

func TestAtomStartDependencyPerVersion(t *testing.T) {
	fa := afake.NewSimpleClientset(testAtom("Pending", "app-2"))
	a := testAtomController(fa)

	a.dependencyProcessor.Store(dependencyKey(testAtom("Pending", "app-1")), true)

	a.startDependency(testAtom("Pending", "app-2"))

	require.Eventually(t, func() bool {
		live, err := fa.AtomV1().Atoms("rack1-app1").Get(context.Background(), "app", am.GetOptions{})
		return err == nil && live.Spec.Dependencies == nil
	}, 5*time.Second, 10*time.Millisecond)
}

func TestAtomStartDependencyNotPending(t *testing.T) {
	fa := afake.NewSimpleClientset(testAtom("Cancelled", "app-1"))

	testAtomController(fa).startDependency(testAtom("Cancelled", "app-1"))

	require.Never(t, func() bool { return len(fa.Actions()) > 0 }, 200*time.Millisecond, 10*time.Millisecond)
}

func TestAtomSyncAllLeaderOnly(t *testing.T) {
	fa := afake.NewSimpleClientset()
	a := testAtomController(fa)
	a.controller = &kctl.Controller{}

	require.NoError(t, a.syncAll())
	require.Empty(t, fa.Actions())

	a.controller.IsLeader.Store(true)

	require.NoError(t, a.syncAll())
	require.Len(t, fa.Actions(), 1)
}
