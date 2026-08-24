package k8s

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/mock"
	"github.com/stretchr/testify/require"
)

type logCaptureEngine struct {
	*mock.TestEngine

	calls []string
}

func (e *logCaptureEngine) Log(app, stream string, _ time.Time, _ string) error {
	e.calls = append(e.calls, fmt.Sprintf("%s %s", app, stream))
	return nil
}

func TestSystemLogStream(t *testing.T) {
	for _, tc := range []struct {
		gates string
		tid   string
		want  string
	}{
		{gates: "tid=true", tid: "ab12", want: "ab12-app1 system/state"},
		{gates: "tid=true", want: "app1 system/state"},
		{want: "app1 system/k8s/state"},
		{tid: "ab12", want: "ab12-app1 system/k8s/state"},
	} {
		t.Setenv("FEATURE_GATES", tc.gates)

		e := &logCaptureEngine{TestEngine: &mock.TestEngine{}}
		p := &Provider{Engine: e}

		require.NoError(t, p.systemLog(tc.tid, "app1", "state", time.Now(), "hello"))
		require.Equal(t, []string{tc.want}, e.calls)
	}
}

func TestNamespaceTid(t *testing.T) {
	p := &Provider{Name: "rack1"}

	for _, tc := range []struct {
		name string
		ns   string
		app  string
		want string
	}{
		{name: "hyphenated app, no tenant", ns: "rack1-amzn-advertising-staging", app: "amzn-advertising-staging"},
		{name: "hyphenated app, tenant", ns: "rack1-ab12-amzn-advertising-staging", app: "amzn-advertising-staging", want: "ab12"},
		{name: "single token app, no tenant", ns: "rack1-app1", app: "app1"},
		{name: "single token app, tenant", ns: "rack1-ab12-app1", app: "app1", want: "ab12"},
		{name: "same namespace, app named for the leading segment", ns: "rack1-ab12-app1", app: "ab12-app1"},
		{name: "rack system namespace", ns: "rack1-system", app: "system"},
		{name: "namespace without the rack prefix", ns: "kube-system", app: "system"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tid := p.namespaceTid(tc.ns, tc.app)
			require.Equal(t, tc.want, tid)

			if !strings.HasPrefix(tc.ns, p.Name+"-") {
				return
			}

			name := tc.app
			if tid != "" {
				name = fmt.Sprintf("%s-%s", tid, tc.app)
			}
			require.Equal(t, strings.TrimPrefix(tc.ns, p.Name+"-"), name)
		})
	}
}
