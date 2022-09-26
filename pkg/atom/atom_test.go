package atom

import (
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
		a, err := fac.AtomV1().Atoms("ns1").Get("atom1", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, aa.AtomStatus("Cancelled"), a.Status)

		require.NoError(t, ac.Cancel("ns1", "atom2"))
		a, err = fac.AtomV1().Atoms("ns1").Get("atom2", am.GetOptions{})
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
				require.NoError(t, ac.Apply(test.AtomNamespace, test.AtomName, test.AtomRelease, nil, 600))

				a, err := fac.AtomV1().Atoms(test.AtomNamespace).Get(test.AtomName, am.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, aa.AtomStatus("Pending"), a.Status)
			}

			t.Run(test.Name, fn)
		}
	})
}

func atomCreate(ac av.Interface, namespace, name, status, version string, spec aa.AtomSpec) error {
	_, err := ac.AtomV1().Atoms(namespace).Create(&aa.Atom{
		ObjectMeta: am.ObjectMeta{
			Name: name,
		},
		Status: aa.AtomStatus(status),
		Spec:   spec,
	})
	if err != nil {
		return err
	}

	_, err = ac.AtomV1().AtomVersions(namespace).Create(&aa.AtomVersion{
		ObjectMeta: am.ObjectMeta{
			Name: version,
		},
		Spec: aa.AtomVersionSpec{
			Release: version,
		},
	})
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
