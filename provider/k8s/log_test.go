package k8s

import (
	"fmt"
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
