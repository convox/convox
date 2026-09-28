package billing_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/billing"
	"github.com/stretchr/testify/assert"
)

// mirrors prices_azure_test.go guards for the GCP table
func TestGcpPriceTable_SanityFloors(t *testing.T) {
	for k, v := range billing.GcpInstancePricing {
		assert.Less(t, v.OnDemandUsdPerHour, 500.0, "price for %s exceeds sanity ceiling", k)
		assert.GreaterOrEqual(t, v.OnDemandUsdPerHour, 0.001, "price for %s below sanity floor", k)
	}
}

func TestGcpPriceTable_KeyFormat(t *testing.T) {
	re := regexp.MustCompile(`^[a-z][0-9][a-z]?-[a-z0-9-]+$`)
	for k := range billing.GcpInstancePricing {
		assert.True(t, re.MatchString(k), "unexpected key shape: %s", k)
	}
}

func TestGcpPriceTable_EntryMetadata(t *testing.T) {
	gpuTypes := map[string]bool{"L4": true, "A100": true, "H100": true, "H200": true, "B200": true, "RTX PRO 6000": true}
	gpuFamily := regexp.MustCompile(`^(g2-|a2-|a3-|a4|g4-)`)
	for k, v := range billing.GcpInstancePricing {
		assert.Greater(t, v.VcpuCount, 0, "%s missing vcpu", k)
		assert.Greater(t, v.MemGb, 0.0, "%s missing memory", k)
		if gpuFamily.MatchString(k) || v.GpuCount > 0 || v.GpuType != "" {
			assert.GreaterOrEqual(t, v.GpuCount, 1, "%s gpu count", k)
			assert.True(t, gpuTypes[v.GpuType], "%s gpu type %q outside vocabulary", k, v.GpuType)
		}
		if v.SpotUsdPerHourFactor != 0 {
			assert.Greater(t, v.SpotUsdPerHourFactor, 0.0, "%s spot factor", k)
			assert.Less(t, v.SpotUsdPerHourFactor, 1.0, "%s spot factor", k)
		}
	}
}

func TestGcpPriceTable_FamilyCoverage(t *testing.T) {
	families := []string{
		"g2", "a2", "a3", "a4", "g4",
		"e2", "f1", "g1", "n1", "n2", "n2d", "n4", "n4a", "n4d",
		"c3", "c3d", "c4", "c4a", "c4d", "t2a", "t2d",
		"c2", "c2d", "h3", "c4n",
		"m1", "m2", "m3", "m4",
		"z3", "z4d",
	}
	for _, fam := range families {
		found := false
		for k := range billing.GcpInstancePricing {
			if strings.HasPrefix(k, fam+"-") {
				found = true
				break
			}
		}
		assert.True(t, found, "missing price entries for family %s", fam)
	}
}

func TestPriceForInstanceOn_Gcp(t *testing.T) {
	p, ok := billing.PriceForInstanceOn("gcp", "g2-standard-4")
	assert.True(t, ok)
	assert.Equal(t, "L4", p.GpuType)
	assert.Equal(t, 1, p.GpuCount)

	p, ok = billing.PriceForInstanceOn("gcp", "n1-standard-2")
	assert.True(t, ok)
	assert.Equal(t, 0, p.GpuCount)
	assert.Greater(t, p.SpotUsdPerHourFactor, 0.0)

	_, ok = billing.PriceForInstanceOn("gcp", "not-a-real-type")
	assert.False(t, ok)
}

// Every GPU machine family detected by provider/gcp gcpGpuMachinePrefixes
// (g2-, a2-, a3-, a4-, g4-) must have at least one priced representative,
// otherwise labelled GPU nodes silently contribute zero spend to budgets.
func TestGcpGpuFamiliesArePriced(t *testing.T) {
	representatives := map[string]string{
		"g2-": "g2-standard-8",
		"a2-": "a2-highgpu-1g",
		"a3-": "a3-highgpu-8g",
		"a4-": "a4-highgpu-8g",
		"g4-": "g4-standard-48",
	}
	for family, instanceType := range representatives {
		p, ok := billing.PriceForInstanceOn("gcp", instanceType)
		if !ok {
			t.Errorf("GPU family %s has no pricing entry for representative %s", family, instanceType)
			continue
		}
		if p.GpuCount < 1 || p.GpuType == "" {
			t.Errorf("pricing entry %s must carry GPU metadata, got count=%d type=%q", instanceType, p.GpuCount, p.GpuType)
		}
	}
}
