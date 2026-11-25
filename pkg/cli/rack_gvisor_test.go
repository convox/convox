package cli

import (
	"encoding/json"
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeGroupGvisorValidation(t *testing.T) {
	ng := NodeGroupConfigParam{Type: "m6i.xlarge", Label: options.String("sandbox"), Gvisor: options.Bool(true)}
	require.NoError(t, ng.Validate("aws"))

	err := ng.Validate("azure")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported on AWS")

	ng.AmiID = options.String("ami-0123456789abcdef0")
	err = ng.Validate("aws")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "EKS-optimized AMI")

	off := NodeGroupConfigParam{Type: "m6i.xlarge", AmiID: options.String("ami-0123456789abcdef0"), Gvisor: options.Bool(false)}
	require.NoError(t, off.Validate("aws"), "gvisor false changes nothing")
}

func TestNodeGroupGvisorSurvivesReencoding(t *testing.T) {
	var ngs AdditionalNodeGroups
	require.NoError(t, json.Unmarshal([]byte(`[{"id":1,"type":"m6i.xlarge","label":"sandbox","dedicated":true,"gvisor":true}]`), &ngs))
	out, err := json.Marshal(ngs)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"gvisor":true`, "params set re-encodes the JSON, so the field must be on the struct")

	legacy := `[{"id":2,"type":"t3.large","label":"web"}]`
	var old AdditionalNodeGroups
	require.NoError(t, json.Unmarshal([]byte(legacy), &old))
	out, err = json.Marshal(old)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "gvisor", "existing configs re-encode without the new field")
}

func TestKarpenterPoolGvisorValidation(t *testing.T) {
	np := KarpenterNodePoolConfigParam{Name: "sandbox", Gvisor: options.Bool(true)}
	require.NoError(t, np.Validate())

	np.AmiID = options.String("ami-0123456789abcdef0")
	err := np.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default AL2023 AMI")

	var pools KarpenterNodePools
	require.NoError(t, json.Unmarshal([]byte(`[{"name":"sandbox","dedicated":true,"gvisor":true}]`), &pools))
	out, err := json.Marshal(pools)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"gvisor":true`)
}

func TestValidateGvisorNodeOS(t *testing.T) {
	gv := `[{"name":"sandbox","gvisor":true}]`
	plain := `[{"name":"workers"}]`

	require.NoError(t, validateGvisorNodeOS(map[string]string{"additional_karpenter_nodepools_config": gv}, map[string]string{}))
	require.NoError(t, validateGvisorNodeOS(map[string]string{"karpenter_node_os": "bottlerocket"}, map[string]string{"additional_karpenter_nodepools_config": plain}))

	err := validateGvisorNodeOS(map[string]string{"karpenter_node_os": "bottlerocket"}, map[string]string{"additional_karpenter_nodepools_config": gv})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "karpenter_node_os=al2023")

	err = validateGvisorNodeOS(map[string]string{"additional_karpenter_nodepools_config": gv}, map[string]string{"karpenter_node_os": "bottlerocket"})
	require.Error(t, err, "the stored OS counts too")
}
