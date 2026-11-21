package k8s

import (
	"context"
	"testing"
	"time"

	afake "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned/fake"
	"github.com/convox/convox/pkg/mock"
	"github.com/stretchr/testify/require"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

type podIdentityLogEngine struct {
	mock.TestEngine
	messages []string
}

func (e *podIdentityLogEngine) Log(_, _ string, _ time.Time, message string) error {
	e.messages = append(e.messages, message)
	return nil
}

func TestPodIdentityWaitId(t *testing.T) {
	id := podIdentityWaitId("app1", "ab12", time.UnixMilli(1700000000123))

	require.Equal(t, "##|app:app1|type:podidentity-wait|tid:ab12|until:1700000000123|##", id)
	require.Equal(t, &resourceSubstitution{App: "app1", RType: podIdentityWaitType, Tid: "ab12", Until: 1700000000123}, parseResourceSubstitutionId(id))

	require.Zero(t, parseResourceSubstitutionId("##|app:app1|type:podidentity-wait|tid:|until:soon|##").Until)
}

func TestAtomPodIdentityWait(t *testing.T) {
	tests := []struct {
		Name   string
		Until  time.Duration
		NoTime bool
		Cap    time.Duration
		Min    time.Duration
		Logged bool
	}{
		{Name: "holds until the time", Until: 300 * time.Millisecond, Cap: time.Minute, Min: 250 * time.Millisecond, Logged: true},
		{Name: "past time clears at once", Until: -time.Second, Cap: time.Minute},
		{Name: "no time clears at once", NoTime: true, Cap: time.Minute},
		{Name: "far time is capped", Until: time.Hour, Cap: 300 * time.Millisecond, Min: 250 * time.Millisecond, Logged: true},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			prev := podIdentityPropagation
			podIdentityPropagation = test.Cap
			t.Cleanup(func() { podIdentityPropagation = prev })

			obj := testAtom("Pending", "app-1")
			fa := afake.NewSimpleClientset(obj.DeepCopy())

			var cleared time.Time
			fa.PrependReactor("update", "atoms", func(k8stesting.Action) (bool, runtime.Object, error) {
				cleared = time.Now()
				return false, nil, nil
			})

			engine := &podIdentityLogEngine{}
			a := testAtomController(fa)
			a.provider.Engine = engine

			start := time.Now()
			obj.Spec.Dependencies = []string{podIdentityWaitId("app1", "", start.Add(test.Until))}
			if test.NoTime {
				obj.Spec.Dependencies = []string{"##|app:app1|type:podidentity-wait|tid:|##"}
			}

			a.processDependency(obj)

			live, err := fa.AtomV1().Atoms("rack1-app1").Get(context.Background(), "app", am.GetOptions{})
			require.NoError(t, err)
			require.Nil(t, live.Spec.Dependencies)

			require.GreaterOrEqual(t, cleared.Sub(start), test.Min)
			require.Less(t, cleared.Sub(start), 5*time.Second)

			if test.Logged {
				require.Len(t, engine.messages, 1)
				require.Contains(t, engine.messages[0], "pod identity")
			} else {
				require.Empty(t, engine.messages)
			}
		})
	}
}
