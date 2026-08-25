package oci

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGPUIntanceList(t *testing.T) {
	p := &Provider{}

	in := []string{
		"VM.GPU.A10.1",        // A10
		"VM.GPU2.1",           // P100
		"VM.GPU3.4",           // V100
		"BM.GPU.A100-v2.8",    // A100
		"BM.GPU.H100.8",       // H100
		"BM.GPU.B200.8",       // B200
		"BM.GPU.MI300X.8",     // AMD Instinct, not NVIDIA
		"BM.GPU.MI355X.8",     // AMD Instinct, not NVIDIA
		"VM.Standard.E4.Flex", // CPU only
		"BM.Standard3.64",     // CPU only
		"vm.gpu.a10.2",        // case-insensitive
	}

	got, err := p.GPUIntanceList(in)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"VM.GPU.A10.1",
		"VM.GPU2.1",
		"VM.GPU3.4",
		"BM.GPU.A100-v2.8",
		"BM.GPU.H100.8",
		"BM.GPU.B200.8",
		"vm.gpu.a10.2",
	}, got)
}
