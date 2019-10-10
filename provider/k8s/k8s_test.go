package k8s_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

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

	n, err := c.CoreV1().Namespaces().Create(&ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "test"}})
	fmt.Printf("n: %+v\n", n)
	fmt.Printf("err: %+v\n", err)
	require.NoError(t, err)

	os.Setenv("NAMESPACE", "test")

	fn(p)
}

func testProviderManual(t *testing.T, fn func(*k8s.Provider, *fake.Clientset)) {
	c := &fake.Clientset{}

	p := &k8s.Provider{
		Cluster: c,
	}

	fn(p, c)
}
