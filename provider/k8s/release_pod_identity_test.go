package k8s_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	tmock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type podIdentityTestEngine struct {
	mock.TestEngine
	created time.Time
	err     error
	apps    []string
}

func (e *podIdentityTestEngine) SyncPodIdentity(app string, _ *manifest.Manifest) (time.Time, error) {
	e.apps = append(e.apps, app)
	return e.created, e.err
}

const podIdentityTestManifest = "services:\n  web:\n    image: docker.io/library/nginx\n    port: 5000\n"

func TestReleasePromotePodIdentityWait(t *testing.T) {
	tests := []struct {
		Name    string
		Created func() time.Time
		Promote func(p *k8s.Provider) error
		Wait    bool
		Timeout time.Duration
	}{
		{
			Name:    "new association waits",
			Created: time.Now,
			Promote: func(p *k8s.Provider) error {
				return p.ReleasePromote("app1", "release2", structs.ReleasePromoteOptions{})
			},
			Wait:    true,
			Timeout: manifest.DefaultProgressDeadlineSeconds * time.Second,
		},
		{
			Name:    "old association does not wait",
			Created: func() time.Time { return time.Now().Add(-2 * time.Minute) },
			Promote: func(p *k8s.Provider) error {
				return p.ReleasePromote("app1", "release2", structs.ReleasePromoteOptions{})
			},
			Timeout: manifest.DefaultProgressDeadlineSeconds * time.Second,
		},
		{
			Name:    "no association does not wait",
			Created: func() time.Time { return time.Time{} },
			Promote: func(p *k8s.Provider) error {
				return p.ReleasePromote("app1", "release2", structs.ReleasePromoteOptions{})
			},
			Timeout: manifest.DefaultProgressDeadlineSeconds * time.Second,
		},
		{
			Name:    "app update carries the wait",
			Created: time.Now,
			Promote: func(p *k8s.Provider) error {
				return p.AppUpdate("app1", structs.AppUpdateOptions{})
			},
			Wait:    true,
			Timeout: 30 * time.Second,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				engine := &podIdentityTestEngine{created: test.Created()}
				p.Engine = engine

				kk, _ := p.Cluster.(*fake.Clientset)
				cc, _ := p.Convox.(*cvfake.Clientset)
				aa, _ := p.Atom.(*atom.MockInterface)

				require.NoError(t, appCreate(kk, "rack1", "app1"))
				require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", podIdentityTestManifest))
				require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release2", podIdentityTestManifest))
				require.NoError(t, buildCreate(cc, "rack1-app1", "build1", "basic"))

				aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)

				var deps []string
				aa.On("Apply", "rack1-app1", "app", tmock.Anything).Return(nil).Once().Run(func(args tmock.Arguments) {
					cfg, ok := args.Get(2).(*atom.ApplyConfig)
					require.True(t, ok)
					deps = cfg.Dependencies
				})

				require.NoError(t, test.Promote(p))
				require.Equal(t, []string{"app1"}, engine.apps)

				ns, err := kk.CoreV1().Namespaces().Get(context.Background(), "rack1-app1", am.GetOptions{})
				require.NoError(t, err)

				var state structs.ReleasePromoteWatchState
				require.NoError(t, json.Unmarshal([]byte(ns.Annotations[structs.ReleasePromoteWatchAnnotation]), &state))
				window := state.ExpiresAt.Sub(state.StartedAt)

				if !test.Wait {
					require.Empty(t, deps)
					require.InDelta(t, test.Timeout.Seconds(), window.Seconds(), 1)
					return
				}

				require.Len(t, deps, 1)
				require.InDelta(t, (test.Timeout + 90*time.Second).Seconds(), window.Seconds(), 5)

				var until int64
				_, err = fmt.Sscanf(deps[0], "##|app:app1|type:podidentity-wait|tid:|until:%d|##", &until)
				require.NoError(t, err)
				require.WithinDuration(t, engine.created.Add(90*time.Second), time.UnixMilli(until), time.Second)
			})
		})
	}
}

func TestAppUpdateTimeoutDuringRollout(t *testing.T) {
	tests := []struct {
		Status  string
		Timeout int32
	}{
		{Status: "Running", Timeout: 30},
		{Status: "Pending", Timeout: manifest.DefaultProgressDeadlineSeconds},
		{Status: "Updating", Timeout: 30},
	}

	for _, test := range tests {
		t.Run(test.Status, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				kk, _ := p.Cluster.(*fake.Clientset)
				cc, _ := p.Convox.(*cvfake.Clientset)
				aa, _ := p.Atom.(*atom.MockInterface)

				require.NoError(t, appCreate(kk, "rack1", "app1"))
				require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", podIdentityTestManifest))
				require.NoError(t, buildCreate(cc, "rack1-app1", "build1", "basic"))

				aa.On("Status", "rack1-app1", "app").Return(test.Status, "release1", nil)

				var timeout int32
				aa.On("Apply", "rack1-app1", "app", tmock.Anything).Return(nil).Once().Run(func(args tmock.Arguments) {
					cfg, ok := args.Get(2).(*atom.ApplyConfig)
					require.True(t, ok)
					timeout = cfg.Timeout
				})

				require.NoError(t, p.AppUpdate("app1", structs.AppUpdateOptions{}))
				require.Equal(t, test.Timeout, timeout)
			})
		})
	}
}

func TestReleasePromotePodIdentityError(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		p.Engine = &podIdentityTestEngine{err: errors.New("failed to attach the policy: denied")}

		kk, _ := p.Cluster.(*fake.Clientset)
		cc, _ := p.Convox.(*cvfake.Clientset)
		aa, _ := p.Atom.(*atom.MockInterface)

		require.NoError(t, appCreate(kk, "rack1", "app1"))
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", podIdentityTestManifest))
		require.NoError(t, buildCreate(cc, "rack1-app1", "build1", "basic"))

		state := &ac.Secret{ObjectMeta: am.ObjectMeta{Name: "removed-database", Labels: map[string]string{"system": "convox", "type": "state"}}}
		_, err := kk.CoreV1().Secrets("rack1-app1").Create(context.Background(), state, am.CreateOptions{})
		require.NoError(t, err)

		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)

		err = p.ReleasePromote("app1", "release1", structs.ReleasePromoteOptions{})
		require.EqualError(t, err, "failed to attach the policy: denied")
		aa.AssertNotCalled(t, "Apply", tmock.Anything, tmock.Anything, tmock.Anything)

		_, err = kk.CoreV1().Secrets("rack1-app1").Get(context.Background(), "removed-database", am.GetOptions{})
		require.NoError(t, err)
	})
}
