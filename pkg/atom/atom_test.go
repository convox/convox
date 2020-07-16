package atom_test

import (
	"testing"

	"github.com/convox/convox/pkg/atom"
	aa "github.com/convox/convox/pkg/atom/pkg/apis/atom/v1"
	av "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned"
	afake "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCancel(t *testing.T) {
	testClient(t, func(ac *atom.Client) {
		fac := ac.Atom.(*afake.Clientset)

		require.NoError(t, atomCreate(fac, "ns1", "atom1", "Updating"))
		require.NoError(t, atomCreate(fac, "ns1", "atom2", "Rollback"))
		require.NoError(t, atomCreate(fac, "ns1", "atom3", "Other"))

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

func atomCreate(ac av.Interface, namespace, name, status string) error {
	_, err := ac.AtomV1().Atoms(namespace).Create(&aa.Atom{
		ObjectMeta: am.ObjectMeta{
			Name: name,
		},
		Status: aa.AtomStatus(status),
	})
	if err != nil {
		return err
	}

	return nil
}

func testClient(t *testing.T, fn func(*atom.Client)) {
	fa := afake.NewSimpleClientset()

	a := &atom.Client{
		Atom: fa,
	}

	fn(a)
}
