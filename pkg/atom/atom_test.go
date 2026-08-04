package atom

import (
	"context"
	"fmt"
	"testing"

	aa "github.com/convox/convox/pkg/atom/pkg/apis/atom/v1"
	av "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned"
	afake "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestStatus(t *testing.T) {
	tests := []struct {
		Name          string
		AtomNamespace string
		AtomName      string
		AtomStatus    string
		AtomRelease   string
		AtomVersion   string
		AtomSpec      aa.AtomSpec
	}{
		{
			Name:          "Success",
			AtomNamespace: "ns1",
			AtomName:      "atom1",
			AtomStatus:    "Updating",
			AtomRelease:   "",
			AtomSpec:      aa.AtomSpec{},
			AtomVersion:   "v1",
		},
		{
			Name:          "With Current Version",
			AtomNamespace: "ns2",
			AtomName:      "atom2",
			AtomStatus:    "Updating",
			AtomRelease:   "v1.0.0",
			AtomSpec: aa.AtomSpec{
				CurrentVersion: "v1.0.0",
			},
			AtomVersion: "v2",
		},
	}

	testClient(t, func(ac *Client) {
		fac := ac.Atom.(*afake.Clientset)

		for _, test := range tests {
			fn := func(t *testing.T) {
				version := test.AtomVersion
				if test.AtomSpec.CurrentVersion != "" {
					version = test.AtomSpec.CurrentVersion
				}

				require.NoError(t, atomCreate(
					fac,
					test.AtomNamespace,
					test.AtomName,
					test.AtomStatus,
					version,
					test.AtomSpec,
				))

				st, release, err := ac.Status(test.AtomNamespace, test.AtomName)
				assert.Equal(t, test.AtomStatus, st)
				assert.Equal(t, test.AtomRelease, release)
				require.NoError(t, err)
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestCancel(t *testing.T) {
	testClient(t, func(ac *Client) {
		fac := ac.Atom.(*afake.Clientset)

		require.NoError(t, atomCreate(fac, "ns1", "atom1", "Updating", "atom1", aa.AtomSpec{}))
		require.NoError(t, atomCreate(fac, "ns1", "atom2", "Rollback", "atom2", aa.AtomSpec{}))
		require.NoError(t, atomCreate(fac, "ns1", "atom3", "Other", "atom3", aa.AtomSpec{}))

		require.NoError(t, ac.Cancel("ns1", "atom1"))
		a, err := fac.AtomV1().Atoms("ns1").Get(context.Background(), "atom1", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, aa.AtomStatus("Cancelled"), a.Status)

		require.NoError(t, ac.Cancel("ns1", "atom2"))
		a, err = fac.AtomV1().Atoms("ns1").Get(context.Background(), "atom2", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, aa.AtomStatus("Failure"), a.Status)

		err = ac.Cancel("ns1", "atom3")
		require.EqualError(t, err, "not currently updating")
	})
}

func TestApply(t *testing.T) {
	tests := []struct {
		Name          string
		AtomNamespace string
		AtomName      string
		AtomRelease   string
	}{
		{
			Name:          "Success",
			AtomNamespace: "ns1",
			AtomName:      "atom1",
			AtomRelease:   "1.0",
		},
	}

	testClient(t, func(ac *Client) {
		fac := ac.Atom.(*afake.Clientset)

		for _, test := range tests {
			fn := func(t *testing.T) {
				require.NoError(t, ac.Apply(test.AtomNamespace, test.AtomName, &ApplyConfig{
					Release:  test.AtomRelease,
					Template: nil,
					Timeout:  600,
				}))

				a, err := fac.AtomV1().Atoms(test.AtomNamespace).Get(context.Background(), test.AtomName, am.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, aa.AtomStatus("Pending"), a.Status)
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestIsRecoverableApplyError(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{
			name: "duplicate service port strategic-merge delete (k8s #105610)",
			out: `Error from server (Invalid): error when applying patch:` +
				`{"spec":{"$setElementOrder/ports":[{"port":8080}],"ports":[{"$patch":"delete","name":"main","port":8080}]}}` +
				` to:` + "\n" +
				`for: "STDIN": error when patching "STDIN": Service "web" is invalid: spec.ports: Required value`,
			want: true,
		},
		{
			name: "immutable field",
			out:  `The Service "web" is invalid: spec.clusterIP: Invalid value: "": field is immutable`,
			want: true,
		},
		{
			name: "genuinely invalid object must NOT force-recreate",
			out:  `The Service "web" is invalid: spec.ports[0].port: Invalid value: 99999999: must be between 1 and 65535`,
			want: false,
		},
		{
			name: "patch failure without delete directive must NOT force-recreate",
			out: `Error from server: error when applying patch:` +
				`{"spec":{"$setElementOrder/ports":[{"port":8080}]}}` +
				` to:` + "\n" +
				`for: "STDIN": error when patching "STDIN": Service "web" is invalid: spec.ports: Required value`,
			want: false,
		},
		{
			name: "clean apply output",
			out:  "service/web configured\n",
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRecoverableApplyError([]byte(tc.out)); got != tc.want {
				t.Fatalf("isRecoverableApplyError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStatefulSetReady(t *testing.T) {
	tests := []struct {
		name       string
		desired    int
		generation int
		observed   int
		replicas   int
		ready      int
		available  int
		updated    int
		current    string
		update     string
		want       bool
	}{
		{name: "ready", desired: 3, generation: 2, observed: 2, replicas: 3, ready: 3, available: 3, updated: 3, current: "rev2", update: "rev2", want: true},
		{name: "pvc or pod pending", desired: 3, generation: 2, observed: 2, replicas: 2, ready: 1, available: 1, updated: 2, current: "rev1", update: "rev2"},
		{name: "controller has not observed update", desired: 3, generation: 3, observed: 2, replicas: 3, ready: 3, available: 3, updated: 3, current: "rev3", update: "rev3"},
		{name: "rolling update incomplete", desired: 3, generation: 3, observed: 3, replicas: 3, ready: 3, available: 3, updated: 2, current: "rev2", update: "rev3"},
		{name: "scaled to zero", desired: 0, generation: 4, observed: 4, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{"metadata":{"generation":%d},"spec":{"replicas":%d},"status":{"observedGeneration":%d,"replicas":%d,"readyReplicas":%d,"availableReplicas":%d,"updatedReplicas":%d,"currentRevision":%q,"updateRevision":%q}}`,
				tt.generation, tt.desired, tt.observed, tt.replicas, tt.ready, tt.available, tt.updated, tt.current, tt.update))
			got, err := statefulSetReady(data)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestExtractStatefulSetConditions(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  namespace: app
  name: database
  annotations:
    atom.conditions: Ready=True
`)
	conditions, err := extractConditions(data)
	require.NoError(t, err)
	require.Len(t, conditions, 1)
	require.Equal(t, "StatefulSet", conditions[0].Kind)
	require.Equal(t, "database", conditions[0].Name)
}

func atomCreate(ac av.Interface, namespace, name, status, version string, spec aa.AtomSpec) error {
	_, err := ac.AtomV1().Atoms(namespace).Create(context.Background(), &aa.Atom{
		ObjectMeta: am.ObjectMeta{
			Name: name,
		},
		Status: aa.AtomStatus(status),
		Spec:   spec,
	}, am.CreateOptions{})
	if err != nil {
		return err
	}

	_, err = ac.AtomV1().AtomVersions(namespace).Create(context.Background(), &aa.AtomVersion{
		ObjectMeta: am.ObjectMeta{
			Name: version,
		},
		Spec: aa.AtomVersionSpec{
			Release: version,
		},
	}, am.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func testClient(t *testing.T, fn func(*Client)) {
	fa := afake.NewSimpleClientset()
	c := fake.NewSimpleClientset()

	a := &Client{
		Atom: fa,
		k8s:  c,
	}

	fn(a)
}
