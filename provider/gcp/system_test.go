package gcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGPUIntanceList(t *testing.T) {
	p := &Provider{}

	in := []string{
		"g2-standard-4",  // L4
		"a2-highgpu-1g",  // A100
		"a2-megagpu-16g", // A100
		"a3-highgpu-8g",  // H100
		"a4-highgpu-8g",  // B200
		"g4-standard-48", // RTX PRO 6000
		"n1-standard-4",  // GPU-attachable but undetectable by name
		"e2-standard-4",  // CPU only
		"n2-standard-8",  // CPU only
		"c3-highcpu-176", // CPU only
		"G2-STANDARD-4",  // case-insensitive
	}

	got, err := p.GPUIntanceList(in)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"g2-standard-4",
		"a2-highgpu-1g",
		"a2-megagpu-16g",
		"a3-highgpu-8g",
		"a4-highgpu-8g",
		"g4-standard-48",
		"G2-STANDARD-4",
	}, got)
}

func TestGPUIntanceListEmpty(t *testing.T) {
	p := &Provider{}
	got, err := p.GPUIntanceList(nil)
	require.NoError(t, err)
	require.Empty(t, got)
}
