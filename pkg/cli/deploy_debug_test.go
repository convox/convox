package cli_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func fxDiagnosticReport() *structs.AppDiagnosticReport {
	return &structs.AppDiagnosticReport{
		Namespace: "myrack-app1",
		Rack:      "myrack",
		App:       "app1",
		Timestamp: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
		Overview: &structs.DiagnosticOverview{
			Services: []structs.DiagnosticServiceStatus{
				{
					Name:            "web",
					DesiredReplicas: 2,
					ReadyReplicas:   2,
					UpdatedReplicas: 2,
					Status:          "running",
				},
				{
					Name:            "worker",
					DesiredReplicas: 2,
					ReadyReplicas:   0,
					UpdatedReplicas: 2,
					Status:          "deploying",
				},
			},
		},
		Pods: []structs.DiagnosticPod{
			{
				Name:           "worker-abc123",
				Service:        "worker",
				Phase:          "Running",
				Ready:          "0/1",
				AgeSeconds:     180,
				Restarts:       5,
				Classification: "not-ready",
				StateDetail:    "CrashLoopBackOff",
				Hint:           "Process is crash-looping on startup -- check the logs below for the error",
				Logs:           "Error: connect ECONNREFUSED 10.0.2.15:5432\n",
			},
		},
		Summary: &structs.DiagnosticSummary{
			Total:     3,
			Unhealthy: 0,
			NotReady:  1,
			New:       0,
			Healthy:   2,
		},
	}
}

func TestDeployDebug(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppDiagnose", "app1", mock.Anything).Return(fxDiagnosticReport(), nil)

		res, err := testExecute(e, "deploy-debug -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "Deploy Diagnostics:")
		require.Contains(t, res.Stdout, "app1")
		require.Contains(t, res.Stdout, "worker")
		require.Contains(t, res.Stdout, "CrashLoopBackOff")
		require.Contains(t, res.Stdout, "crash-looping")
		require.Contains(t, res.Stdout, "ECONNREFUSED")
	})
}

func TestDeployDebugJSON(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppDiagnose", "app1", mock.Anything).Return(fxDiagnosticReport(), nil)

		res, err := testExecute(e, "deploy-debug -a app1 -o json", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, `"namespace": "myrack-app1"`)
		require.Contains(t, res.Stdout, `"classification": "not-ready"`)
		require.Contains(t, res.Stdout, `"CrashLoopBackOff"`)
	})
}

func TestDeployDebugSummary(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppDiagnose", "app1", mock.Anything).Return(fxDiagnosticReport(), nil)

		res, err := testExecute(e, "deploy-debug -a app1 -o summary", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "SERVICE")
		require.Contains(t, res.Stdout, "worker")
		require.Contains(t, res.Stdout, "PROCESS")
		require.Contains(t, res.Stdout, "worker-abc123")
	})
}

func TestDeployDebugEmpty(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		report := &structs.AppDiagnosticReport{
			Namespace: "myrack-app1",
			Rack:      "myrack",
			App:       "app1",
			Timestamp: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
			Summary: &structs.DiagnosticSummary{
				Total:   2,
				Healthy: 2,
			},
		}
		i.On("AppDiagnose", "app1", mock.Anything).Return(report, nil)

		res, err := testExecute(e, "deploy-debug -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "healthy")
	})
}

func TestDeployDebugError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppDiagnose", "app1", mock.Anything).Return(nil, fmt.Errorf("namespace myrack-app1 not found"))

		res, err := testExecute(e, "deploy-debug -a app1", nil)
		require.NoError(t, err)
		require.NotEqual(t, 0, res.Code)
		require.Contains(t, res.Stderr, "not found")
	})
}
