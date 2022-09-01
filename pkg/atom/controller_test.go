package atom

import (
	"testing"
	"time"

	aa "github.com/convox/convox/pkg/atom/pkg/apis/atom/v1"
	afake "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned/fake"
	"github.com/convox/convox/pkg/kctl"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestClient(t *testing.T) {
	testController(t, func(ac *AtomController) {
		assert.NotNil(t, ac.Client())
	})
}

func TestStart(t *testing.T) {
	testController(t, func(ac *AtomController) {
		assert.Nil(t, ac.Start())
	})
}

func TestStop(t *testing.T) {
	testController(t, func(ac *AtomController) {
		assert.Nil(t, ac.Stop())
	})
}

func TestAdd(t *testing.T) {
	testController(t, func(ac *AtomController) {
		assert.Nil(t, ac.Add(5))
	})
}

func TestDelete(t *testing.T) {
	testController(t, func(ac *AtomController) {
		assert.Nil(t, ac.Delete(5))
	})
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		Name          string
		AtomNamespace string
		AtomName      string
		AtomStatus    string
		AtomVersion   string
		AtomSpec      aa.AtomSpec
		Prev          interface{}
		Curr          *aa.Atom
		Err           error
	}{
		{
			Name:          "Success",
			AtomNamespace: "ns1",
			AtomName:      "atom1",
			AtomStatus:    "Updating",
			AtomVersion:   "v1",
			AtomSpec:      aa.AtomSpec{},
			Prev:          &aa.Atom{},
			Curr:          &aa.Atom{},
			Err:           nil,
		},
		{
			Name:          "Success - Different Status - Failure",
			AtomNamespace: "ns2",
			AtomName:      "atom2",
			AtomStatus:    "Failure",
			AtomVersion:   "v2",
			Prev: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom2",
				},
				Status: aa.AtomStatus("Pending"),
			},
			Curr: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom2",
				},
				Status: aa.AtomStatus("Failure"),
			},
			Err: nil,
		},
		{
			Name:          "Success - Different Status - Success",
			AtomNamespace: "ns3",
			AtomName:      "atom3",
			AtomStatus:    "Success",
			AtomVersion:   "v3",
			Prev: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom3",
				},
				Status: aa.AtomStatus("Pending"),
			},
			Curr: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom3",
				},
				Status: aa.AtomStatus("Success"),
			},
			Err: nil,
		},
		{
			Name:          "Different Status - Rollback - Deadline Error",
			AtomNamespace: "atom.controller",
			AtomName:      "atom4",
			AtomStatus:    "Failure",
			AtomVersion:   "v4",
			Prev: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom4",
				},
				Status: aa.AtomStatus("Running"),
			},
			Curr: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom4",
				},
				Status: aa.AtomStatus("Rollback"),
			},
			Err: nil,
		},
		{
			Name:          "Different Status - Rollback - Reverted",
			AtomNamespace: "atom.controller",
			AtomName:      "atom5",
			AtomStatus:    "Reverted",
			AtomVersion:   "v.atom5",
			Prev: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom5",
				},
				Status: aa.AtomStatus("Running"),
				Spec: aa.AtomSpec{
					CurrentVersion: "v.atom5",
				},
			},
			Curr: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom5",
				},
				Status:  aa.AtomStatus("Rollback"),
				Started: metav1.Time{Time: time.Now()},
				Spec: aa.AtomSpec{
					CurrentVersion:          "v.atom5",
					ProgressDeadlineSeconds: 180,
				},
			},
			Err: nil,
		},
		{
			Name:          "Updating - Deadline",
			AtomNamespace: "atom.controller",
			AtomName:      "atom6",
			AtomStatus:    "Deadline",
			AtomVersion:   "v6",
			Prev: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom6",
				},
				Status: aa.AtomStatus("Updating"),
			},
			Curr: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom6",
				},
				Status: aa.AtomStatus("Updating"),
			},
			Err: nil,
		},
		{
			Name:          "Updating - Running",
			AtomNamespace: "atom.controller",
			AtomName:      "atom7",
			AtomStatus:    "Running",
			AtomVersion:   "v.atom7",
			Prev: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom7",
				},
				Status: aa.AtomStatus("Updating"),
				Spec: aa.AtomSpec{
					CurrentVersion: "v.atom7",
				},
			},
			Curr: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom5",
				},
				Status:  aa.AtomStatus("Updating"),
				Started: metav1.Time{Time: time.Now()},
				Spec: aa.AtomSpec{
					CurrentVersion:          "v.atom5",
					ProgressDeadlineSeconds: 180,
				},
			},
			Err: nil,
		},
		{
			Name:          "Cancelled - Failure",
			AtomNamespace: "atom.controller",
			AtomName:      "atom8",
			AtomStatus:    "Failure",
			AtomVersion:   "v8",
			Prev: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom8",
				},
				Status: aa.AtomStatus("Cancelled"),
			},
			Curr: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom8",
				},
				Status: aa.AtomStatus("Cancelled"),
			},
			Err: nil,
		},
		{
			Name:          "Pending - Error",
			AtomNamespace: "atom.controller",
			AtomName:      "atom9",
			AtomStatus:    "Error",
			AtomVersion:   "v9",
			Prev: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom9",
				},
				Status: aa.AtomStatus("Pending"),
			},
			Curr: &aa.Atom{
				ObjectMeta: am.ObjectMeta{
					Name: "atom9",
				},
				Status: aa.AtomStatus("Pending"),
			},
			Err: nil,
		},
		{
			Name:          "Wrong atom assert",
			AtomNamespace: "nserr1",
			AtomName:      "atomerr1",
			AtomStatus:    "Updating",
			Prev:          afake.NewSimpleClientset(),
			Curr:          &aa.Atom{},
			Err:           errors.New("could not assert atom for type: *fake.Clientset"),
		},
	}

	testController(t, func(ac *AtomController) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				require.NoError(t, atomCreate(
					ac.convox,
					test.AtomNamespace,
					test.AtomName,
					test.AtomStatus,
					test.AtomVersion,
					test.AtomSpec,
				))

				test.Curr.Namespace = test.AtomNamespace

				err := ac.Update(test.Prev, test.Curr)
				if test.Err == nil {
					require.NoError(t, err)

					a, err := ac.convox.AtomV1().Atoms(test.AtomNamespace).Get(test.AtomName, am.GetOptions{})
					require.NoError(t, err)
					require.Equal(t, aa.AtomStatus(test.AtomStatus), a.Status)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func testController(t *testing.T, fn func(*AtomController)) {
	fa := afake.NewSimpleClientset()
	fakeK8s := fake.NewSimpleClientset()

	client := &Client{
		Atom:   fa,
		config: &rest.Config{},
	}

	fac := client.Atom.(*afake.Clientset)

	a := &AtomController{
		atom:       client,
		convox:     fac,
		kubernetes: fakeK8s,
	}

	c, _ := kctl.NewController("kube-system", "convox-atom", a)

	a.controller = c

	fn(a)
}
