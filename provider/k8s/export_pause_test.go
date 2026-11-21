package k8s

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func ResumeManualPausesForTest(p *Provider, app string) {
	p.resumeManualPauses(app)
}

func RenderedReplicasForTest(t *testing.T, data []byte, name string) int32 {
	t.Helper()
	for _, doc := range bytes.Split(data, []byte("---\n")) {
		var obj struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Replicas *int32 `json:"replicas"`
			} `json:"spec"`
		}
		require.NoError(t, yaml.Unmarshal(doc, &obj))
		if obj.Kind == "Deployment" && obj.Metadata.Name == name {
			require.NotNil(t, obj.Spec.Replicas)
			return *obj.Spec.Replicas
		}
	}
	require.FailNow(t, "no Deployment rendered", name)
	return 0
}
