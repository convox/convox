package k8s

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/kubectl/pkg/cmd/logs"
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

func TestConfigureLogOptionsBySelectorTail(t *testing.T) {
	for _, tc := range []struct {
		name  string
		opts  structs.LogsOptions
		tail  *int64
		since *int64
	}{
		{name: "since without tail", opts: structs.LogsOptions{Since: options.Duration(2 * time.Hour)}, since: options.Int64(7200)},
		{name: "since with tail", opts: structs.LogsOptions{Since: options.Duration(2 * time.Minute), Tail: options.Int(50)}, tail: options.Int64(50), since: options.Int64(120)},
		{name: "tail -1", opts: structs.LogsOptions{Tail: options.Int(-1)}},
		{name: "tail 0", opts: structs.LogsOptions{Tail: options.Int(0)}, tail: options.Int64(0)},
		{name: "since 0", opts: structs.LogsOptions{Since: options.Duration(0)}},
	} {
		for _, follow := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s follow=%t", tc.name, follow), func(t *testing.T) {
				p := &Provider{Name: "rack1", logger: logger.Discard}
				o := logs.NewLogsOptions(genericiooptions.IOStreams{})
				o.Object = &corev1.PodList{}

				opts := tc.opts
				opts.Follow = options.Bool(follow)

				require.NoError(t, p.configureLogOptionsBySelector("app1", "app=app1,name=web,type=service", nil, o, opts))

				lo, ok := o.Options.(*corev1.PodLogOptions)
				require.True(t, ok)
				require.Equal(t, follow, lo.Follow)
				require.Equal(t, tc.tail, lo.TailLines)
				require.Equal(t, tc.since, lo.SinceSeconds)
			})
		}
	}
}
