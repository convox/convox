package atom

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSplitDocuments(t *testing.T) {
	tests := []struct {
		Name string
		In   string
		Out  []string
	}{
		{"Boundary", "a: 1\n---\nb: 2\n", []string{"a: 1\n", "b: 2\n"}},
		{"IndentedBlock", "a: |\n  x\n  ---\n  y\n---\nb: 2\n", []string{"a: |\n  x\n  ---\n  y\n", "b: 2\n"}},
		{"ValueEnding", "key: see ---\nb: 2\n", []string{"key: see ---\nb: 2\n"}},
		{"Placeholder", "##|app:x|##---\nb: 2\n", []string{"##|app:x|##", "b: 2\n"}},
		{"IndentedComment", "a: |\n    # x ---\n    y\n", []string{"a: |\n    # x ---\n    y\n"}},
		{"Trailing", "a: 1\n---\n", []string{"a: 1\n", ""}},
		{"Empty", "", []string{""}},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := []string{}
			for _, p := range SplitDocuments([]byte(test.In)) {
				got = append(got, string(p))
			}
			require.Equal(t, test.Out, got)
		})
	}
}

func TestSplitDocumentsMatchesBoundaries(t *testing.T) {
	stream := []byte("apiVersion: v1\nkind: Namespace\n---\n---\napiVersion: v1\nkind: ConfigMap\ndata:\n  a: |\n    x\n---\n##|app:a|type:rds-postgres|resource:db|stateId:x|tid:|##---\napiVersion: v1\nkind: Service\n")

	require.Equal(t, bytes.Split(stream, []byte("---\n")), SplitDocuments(stream))
}

func TestApplyTemplateKeepsBlockScalar(t *testing.T) {
	calls := stubKubectl(t, nil, nil)
	data := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\ndata:\n  k: |-\n    x\n    ---\n    kind: ClusterRole\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n")

	c := &Client{k8s: fake.NewSimpleClientset()}
	_, err := c.applyTemplate("ns1", data, "atom=abc")
	require.NoError(t, err)
	require.Len(t, *calls, 1)

	docs := SplitDocuments((*calls)[0].data)
	require.Len(t, docs, 2)

	var v struct{ Data map[string]string }
	require.NoError(t, yaml.Unmarshal(docs[0], &v))
	require.Equal(t, "x\n---\nkind: ClusterRole", v.Data["k"])
}

func TestExtractConditionsBlockScalar(t *testing.T) {
	data := []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n  annotations:\n    atom.conditions: Available=True\n    note: |-\n      a\n      ---\n      b\n")

	cs, err := extractConditions(data)
	require.NoError(t, err)
	require.Len(t, cs, 1)
}
