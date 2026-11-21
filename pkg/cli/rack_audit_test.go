package cli_test

import (
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

const auditWarning = "WARNING: ip is the caller's address only on AWS racks with proxy_protocol=true or router_type=contour, otherwise a node or load balancer address"

func auditSystem(version, provider string, params map[string]string) *structs.System {
	s := fxSystem()
	s.Version = version
	s.Provider = provider
	s.Parameters = params
	return s
}

func auditOpts(since time.Duration, follow bool) structs.LogsOptions {
	return structs.LogsOptions{Since: options.Duration(since), Follow: options.Bool(follow)}
}

var fxAuditRecords = []string{
	`{"rackaudit":"api","time":"2026-11-20T10:00:00Z","id":"a1","op":"AppList","method":"GET","path":"/apps","status":200,"auth":"jwt","role":"r","actor":"system-read","token":"0123456789ab","ip":"203.0.113.9","peer":"10.0.3.4"}`,
	`{"rackaudit":"exec","time":"2026-11-20T10:00:01Z","id":"a2","op":"ProcessExec","method":"GET","path":"/apps/web/processes/web-1/exec","status":101,"auth":"password","role":"rwa","actor":"alice@example.com","ip":"198.51.100.7","peer":"10.0.3.4","command":"bash"}`,
	`{"rackaudit":"token","time":"2026-11-20T10:00:02Z","id":"a3","op":"SystemJwtToken","method":"POST","path":"/system/jwt/token","status":200,"auth":"password","role":"rwa","actor":"rack-password","ip":"198.51.100.7","peer":"10.0.3.4","minted":"ba9876543210","granted":"admin","hours":8760}`,
	`{"rackaudit":"kubernetes","time":"2026-11-20T10:00:03Z","op":"Kubernetes","method":"GET","path":"/kubernetes/api/v1/namespaces","auth":"jwt","role":"rwa","actor":"system-admin","token":"ba9876543210","ip":"10.0.9.9","peer":"10.0.9.9"}`,
	`not a record`,
}

var fxAuditOutput = []string{
	`2026-11-20T10:00:00Z api 200 GET "/apps" auth=jwt role=r actor="system-read" token=0123456789ab ip=203.0.113.9 peer=10.0.3.4`,
	`2026-11-20T10:00:01Z exec 101 GET "/apps/web/processes/web-1/exec" auth=password role=rwa actor="alice@example.com" (claimed) ip=198.51.100.7 peer=10.0.3.4 command="bash"`,
	`2026-11-20T10:00:02Z token 200 POST "/system/jwt/token" auth=password role=rwa actor="rack-password" ip=198.51.100.7 peer=10.0.3.4 minted=ba9876543210 granted=admin hours=8760`,
	`2026-11-20T10:00:03Z kubernetes - GET "/kubernetes/api/v1/namespaces" auth=jwt role=rwa actor="system-admin" token=ba9876543210 ip=10.0.9.9 peer=10.0.9.9`,
}

func TestRackAudit(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(auditSystem("3.26.1", "aws", map[string]string{"proxy_protocol": "true"}), nil)
		i.On("SystemAuditLogs", auditOpts(24*time.Hour, false)).Return(testLogs(fxAuditRecords), nil)

		res, err := testExecute(e, "rack audit", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, fxAuditOutput)
	})
}

func TestRackAuditSinceFollow(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(auditSystem("3.26.1-rc1", "aws", map[string]string{"router_type": "contour"}), nil)
		i.On("SystemAuditLogs", auditOpts(time.Hour, true)).Return(testLogs(fxAuditRecords[:1]), nil)

		res, err := testExecute(e, "rack audit --since 1h --follow", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, fxAuditOutput[:1])
	})
}

func TestRackAuditRackTooOld(t *testing.T) {
	for _, version := range []string{"3.26.0", "3.25.14", "20250101000000"} {
		t.Run(version, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("SystemGet").Return(auditSystem(version, "aws", nil), nil)

				res, err := testExecute(e, "rack audit", nil)
				require.NoError(t, err)
				require.Equal(t, 1, res.Code)
				res.RequireStderr(t, []string{"ERROR: convox rack audit needs rack version 3.26.1 or later"})
				i.AssertNotCalled(t, "SystemAuditLogs", auditOpts(24*time.Hour, false))
			})
		})
	}
}

func TestRackAuditSourceWarning(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		params   map[string]string
	}{
		{"aws default", "aws", map[string]string{"proxy_protocol": "false", "router_type": "nginx"}},
		{"aws no params", "aws", nil},
		{"gcp", "gcp", map[string]string{"proxy_protocol": "true"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("SystemGet").Return(auditSystem("3.26.1", tc.provider, tc.params), nil)
				i.On("SystemAuditLogs", auditOpts(24*time.Hour, false)).Return(testLogs(fxAuditRecords[:1]), nil)

				res, err := testExecute(e, "rack audit", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				res.RequireStderr(t, []string{auditWarning})
				res.RequireStdout(t, fxAuditOutput[:1])
			})
		})
	}
}

func TestRackAuditFilters(t *testing.T) {
	cases := []struct {
		args string
		want []string
	}{
		{"--type exec", fxAuditOutput[1:2]},
		{"--actor rack-password", fxAuditOutput[2:3]},
		{"--ip 10.0.3.4", fxAuditOutput[0:3]},
		{"--ip 198.51.100.7", fxAuditOutput[1:3]},
		{"--token ba9876543210", fxAuditOutput[2:4]},
		{"--type api --ip 203.0.113.9", fxAuditOutput[0:1]},
	}

	for _, tc := range cases {
		t.Run(tc.args, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("SystemGet").Return(auditSystem("3.26.1", "aws", map[string]string{"proxy_protocol": "true"}), nil)
				i.On("SystemAuditLogs", auditOpts(24*time.Hour, false)).Return(testLogs(fxAuditRecords), nil)

				res, err := testExecute(e, "rack audit "+tc.args, nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				res.RequireStdout(t, tc.want)
			})
		})
	}
}

func TestRackAuditQuotesHostileValues(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(auditSystem("3.26.1", "aws", map[string]string{"proxy_protocol": "true"}), nil)
		i.On("SystemAuditLogs", auditOpts(24*time.Hour, false)).Return(testLogs([]string{
			`{"rackaudit":"exec","time":"2026-11-20T10:00:00Z","op":"ProcessRun","method":"POST","path":"/apps/\u001b[2Kx/services/web/processes","status":200,"auth":"password","role":"rwa","actor":"rack-password","ip":"10.0.0.1","peer":"10.0.0.1","pid":"web-1","command":"echo \u202eevil\u001b[1A"}`,
			`{"rackaudit":"api","time":"2026-11-20T10:00:01Z","op":"AppList","method":"GET","path":"/apps","status":401,"auth":"none","ip":"10.0.0.1","peer":"10.0.0.1"}`,
		}), nil)

		res, err := testExecute(e, "rack audit", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			`2026-11-20T10:00:00Z exec 200 POST "/apps/\x1b[2Kx/services/web/processes" auth=password role=rwa actor="rack-password" ip=10.0.0.1 peer=10.0.0.1 pid=web-1 command="echo \u202eevil\x1b[1A"`,
			`2026-11-20T10:00:01Z api 401 GET "/apps" auth=none ip=10.0.0.1 peer=10.0.0.1`,
		})
	})
}

func TestRackAuditStreamError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(auditSystem("3.26.1", "aws", map[string]string{"proxy_protocol": "true"}), nil)
		i.On("SystemAuditLogs", auditOpts(24*time.Hour, false)).Return(testLogs([]string{fxAuditRecords[0], "error: AccessDeniedException: not authorized"}), nil)

		res, err := testExecute(e, "rack audit", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStdout(t, fxAuditOutput[:1])
		res.RequireStderr(t, []string{"ERROR: AccessDeniedException: not authorized"})
	})
}

func TestRackAuditRefused(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(auditSystem("3.26.1", "aws", map[string]string{"proxy_protocol": "true"}), nil)
		i.On("SystemAuditLogs", auditOpts(24*time.Hour, false)).Return(testLogs([]string{"ERROR: admin role required"}), nil)

		res, err := testExecute(e, "rack audit", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: admin role required"})
	})
}
