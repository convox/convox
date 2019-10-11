package k8s_test

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAppCancel(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "name1", "app1"))

		aa.On("Status", "name1-app1", "app").Return("Running", "R1234567", nil).Once()
		aa.On("Cancel", "name1-app1", "app").Return(nil).Once()

		err := p.AppCancel("app1")
		require.NoError(t, err)
	})
}

func TestAppCancelMissingApp(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		err := p.AppCancel("app1")
		require.EqualError(t, err, "app not found: app1")
	})
}

func TestAppCancelError(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "name1", "app1"))

		aa.On("Status", "name1-app1", "app").Return("Running", "R1234567", nil).Once()
		aa.On("Cancel", "name1-app1", "app").Return(fmt.Errorf("err1")).Once()

		err := p.AppCancel("app1")
		require.EqualError(t, err, "err1")
	})
}

func TestAppCancelInvalidState(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "name1", "app1"))

		aa.On("Status", "name1-app1", "app").Return("Rollback", "R1234567", nil).Once()
		aa.On("Cancel", "name1-app1", "app").Return(nil).Once()

		err := p.AppCancel("app1")
		require.NoError(t, err)
	})
}

func TestAppCreate(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Apply", "name1-app1", "app", "", mock.Anything, int32(30)).Return(nil).Once().Run(func(args mock.Arguments) {
			requireYamlFixture(t, args.Get(3).([]byte), "app.yml")
			require.NoError(t, appCreate(kk, "name1", "app1"))
		})

		aa.On("Wait", "name1-app1", "app").Return(nil).Once()
		aa.On("Status", "name1-app1", "app").Return("Running", "R1234567", nil).Once()

		a, err := p.AppCreate("app1", structs.AppCreateOptions{})
		require.NoError(t, err)
		require.NotNil(t, a)

		require.Equal(t, "2", a.Generation)
		require.Equal(t, "app1", a.Name)
	})
}

func TestAppDelete(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "name1", "app1"))

		aa.On("Status", "name1-app1", "app").Return("Running", "R1234567", nil).Once()

		err := p.AppDelete("app1")
		require.NoError(t, err)

		_, err = kk.CoreV1().Namespaces().Get("name1-app1", am.GetOptions{})
		require.EqualError(t, err, `namespaces "name1-app1" not found`)
	})
}

func TestAppDeleteMissingApp(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		err := p.AppDelete("app1")
		require.EqualError(t, err, "app not found: app1")
	})
}

func appCreate(c kubernetes.Interface, rack, name string) error {
	_, err := c.CoreV1().Namespaces().Create(&ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", rack, name),
			Labels: map[string]string{
				"name": name,
			},
		},
	})

	return err
}
