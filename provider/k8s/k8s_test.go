package k8s_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func reformatYaml(data []byte) ([]byte, error) {
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
	r1, err := reformatYaml(d1)
	require.NoError(t, err)

	d2, err := ioutil.ReadFile(filepath.Join("testdata", filename))
	require.NoError(t, err)

	r2, err := reformatYaml(d2)
	require.NoError(t, err)

	require.Equal(t, string(r1), string(r2))
}

func testProvider(t *testing.T, fn func(*k8s.Provider)) {
	a := &atom.MockInterface{}
	c := fake.NewSimpleClientset()

	p := &k8s.Provider{
		Atom:      a,
		Cluster:   c,
		Domain:    "domain1",
		Name:      "name1",
		Namespace: "ns1",
	}

	err := p.Initialize(structs.ProviderOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Namespaces().Create(&ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "test"}})
	require.NoError(t, err)

	os.Setenv("NAMESPACE", "test")

	fn(p)

	a.AssertExpectations(t)
}

func testProviderManual(t *testing.T, fn func(*k8s.Provider, *fake.Clientset)) {
	c := &fake.Clientset{}

	p := &k8s.Provider{
		Cluster: c,
	}

	fn(p, c)
}
