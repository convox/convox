package k8s_test

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// func TestAppCancel(t *testing.T) {
//   testProvider(t, func(p *k8s.Provider, c *fake.Clientset) {
//     err := p.AppCancel("app1")
//     require.EqualError(t, err, "unimplemented")
//   })
// }

func remarshalYaml(data []byte) ([]byte, error) {
	rms := [][]byte{}

	parts := bytes.Split(data, []byte("---\n"))

	for _, part := range parts {
		var v interface{}

		if err := yaml.Unmarshal(part, &v); err != nil {
			return nil, err
		}

		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, err
		}

		rms = append(rms, data)
	}

	return bytes.Join(rms, []byte("---\n")), nil
}

func requireYamlFixture(t *testing.T, d1 []byte, filename string) {
	r1, err := remarshalYaml(d1)
	require.NoError(t, err)

	d2, err := ioutil.ReadFile(filepath.Join("testdata", filename))
	require.NoError(t, err)

	r2, err := remarshalYaml(d2)
	require.NoError(t, err)

	require.Equal(t, string(r1), string(r2))
}

func TestAppCreate(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Apply", "name1-app1", "app", "", mock.Anything, int32(30)).Return(nil).Once().Run(func(args mock.Arguments) {
			requireYamlFixture(t, args.Get(3).([]byte), "app.yml")

			_, err := kk.CoreV1().Namespaces().Create(&ac.Namespace{
				ObjectMeta: am.ObjectMeta{
					Name: "name1-app1",
					Labels: map[string]string{
						"name": "app1",
					},
				},
			})
			require.NoError(t, err)
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

// func TestAppCreateError(t *testing.T) {
//   testProviderManual(t, func(p *k8s.Provider, c *fake.Clientset) {
//     c.AddReactor("create", "namespaces", func(action testk8s.Action) (bool, runtime.Object, error) {
//       return true, nil, fmt.Errorf("err1")
//     })

//     a, err := p.AppCreate("app1", structs.AppCreateOptions{})
//     require.EqualError(t, err, "err1")
//     require.Nil(t, a)
//   })
// }

// func TestAppDelete(t *testing.T) {
//   testProvider(t, func(p *k8s.Provider, c *fake.Clientset) {
//     a, err := p.AppCreate("app1", structs.AppCreateOptions{})
//     require.NoError(t, err)
//     require.NotNil(t, a)

//     ns, err := c.CoreV1().Namespaces().Get("test-app1", am.GetOptions{})
//     require.NoError(t, err)
//     require.NotNil(t, ns)

//     err = p.AppDelete("app1")
//     require.NoError(t, err)

//     ns, err = c.CoreV1().Namespaces().Get("test-app1", am.GetOptions{})
//     require.Error(t, err)
//     require.Nil(t, ns)
//   })
// }

// func TestAppDeleteError(t *testing.T) {
//   testProviderManual(t, func(p *k8s.Provider, c *fake.Clientset) {
//     c.AddReactor("delete", "namespaces", func(action testk8s.Action) (bool, runtime.Object, error) {
//       return true, nil, fmt.Errorf("err1")
//     })

//     err := p.AppDelete("app1")
//     require.EqualError(t, err, "err1")
//   })
// }
