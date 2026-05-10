package billing_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/billing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriceForInstance_Hit(t *testing.T) {
	p, ok := billing.PriceForInstance("g5.xlarge")
	require.True(t, ok)
	assert.Equal(t, 1.006, p.OnDemandUsdPerHour)
	assert.Equal(t, 1, p.GpuCount)
	assert.Equal(t, "A10G", p.GpuType)
	assert.Equal(t, 4, p.VcpuCount)
	assert.Equal(t, 16.0, p.MemGb)
}

func TestPriceForInstance_Miss(t *testing.T) {
	_, ok := billing.PriceForInstance("not-a-real-type")
	assert.False(t, ok)
}

func TestPriceForInstance_CpuOnly(t *testing.T) {
	p, ok := billing.PriceForInstance("m5.large")
	require.True(t, ok)
	assert.Equal(t, 0, p.GpuCount)
	assert.Equal(t, "", p.GpuType)
}

func TestPricingTableVersion(t *testing.T) {
	v := billing.PricingTableVersion()
	require.NotEmpty(t, v)
	_, err := time.Parse("2006-01-02", v)
	assert.NoError(t, err, "version must be a date: %s", v)
}

// TestPriceTable_FamilyCoverage asserts at least one entry per canonical
// instance family so a regeneration that accidentally drops a family fails CI.
func TestPriceTable_FamilyCoverage(t *testing.T) {
	families := []string{
		"g4dn", "g5", "g6",
		"p3", "p4d", "p5",
		"inf1", "inf2", "trn1",
		"m5", "c5", "r5",
		"t2", "t3", "t3a",
		"m4", "c4", "r4",
	}

	for _, fam := range families {
		t.Run(fam, func(t *testing.T) {
			prefix := fam + "."
			found := false
			for k := range billing.InstancePricing {
				if strings.HasPrefix(k, prefix) {
					found = true
					break
				}
			}
			assert.True(t, found, "missing price entries for family %s", fam)
		})
	}
}

// TestPriceTable_SanityFloors catches malformed entries — any price above the
// upper-bound canary ($500/hr) or below the lower-bound canary ($0.001/hr)
// is almost certainly a parser bug, not a real AWS rate. The lower bound was
// reduced from $0.01 to $0.001 in 2026-04-29 to admit burstable t-family
// nano/micro instances ($0.0047-$0.0058/hr — real AWS prices).
func TestPriceTable_SanityFloors(t *testing.T) {
	const (
		upperCanary = 500.0
		lowerCanary = 0.001
	)
	for k, v := range billing.InstancePricing {
		assert.Less(t, v.OnDemandUsdPerHour, upperCanary, "price for %s exceeds sanity ceiling", k)
		assert.GreaterOrEqual(t, v.OnDemandUsdPerHour, lowerCanary, "price for %s below sanity floor", k)
	}
}

// TestPriceTable_DriftCheck warns at 120 days, fails at 180.
func TestPriceTable_DriftCheck(t *testing.T) {
	if os.Getenv("SKIP_PRICE_DRIFT_CHECK") == "true" {
		t.Skip("drift check skipped by env")
	}

	v := billing.PricingTableVersion()
	last, err := time.Parse("2006-01-02", v)
	require.NoError(t, err, "PricingTableVersion must parse: %s", v)

	age := time.Since(last)
	switch {
	case age > 180*24*time.Hour:
		t.Fatalf("price table is %s old — quarterly refresh overdue (hard fail > 180 days)", age)
	case age > 120*24*time.Hour:
		t.Logf("WARNING: price table is %s old — manual refresh from AWS pricing JSON overdue (soft warn > 120 days)", age)
	}
}

// TestPriceTable_KeyFormat ensures every key follows the "family.size" shape
// that billing lookups assume.
func TestPriceTable_KeyFormat(t *testing.T) {
	re := regexp.MustCompile(`^[a-z0-9]+\.[a-z0-9]+$`)
	for k := range billing.InstancePricing {
		assert.True(t, re.MatchString(k), "unexpected key shape: %s", k)
	}
}

// TestEffectiveUsdPerHour exercises the spot-vs-on-demand multiplier table
// driven by capacityType + per-row SpotUsdPerHourFactor + SpotDefaultFactor.
func TestEffectiveUsdPerHour(t *testing.T) {
	cases := []struct {
		name         string
		price        billing.InstancePrice
		capacityType string
		expected     float64
	}{
		{
			name:         "OnDemand_explicit",
			price:        billing.InstancePrice{OnDemandUsdPerHour: 1.006},
			capacityType: "on-demand",
			expected:     1.006,
		},
		{
			name:         "Empty_capacity_falls_through_to_on_demand",
			price:        billing.InstancePrice{OnDemandUsdPerHour: 1.006},
			capacityType: "",
			expected:     1.006,
		},
		{
			name:         "Unknown_capacity_falls_through_to_on_demand",
			price:        billing.InstancePrice{OnDemandUsdPerHour: 1.006},
			capacityType: "preemptible",
			expected:     1.006,
		},
		{
			name:         "Spot_default_factor_30pct",
			price:        billing.InstancePrice{OnDemandUsdPerHour: 1.006},
			capacityType: "spot",
			expected:     1.006 * billing.SpotDefaultFactor,
		},
		{
			name:         "Spot_per_row_override_45pct",
			price:        billing.InstancePrice{OnDemandUsdPerHour: 1.006, SpotUsdPerHourFactor: 0.45},
			capacityType: "spot",
			expected:     1.006 * 0.45,
		},
		{
			name:         "Spot_zero_factor_falls_back_to_default",
			price:        billing.InstancePrice{OnDemandUsdPerHour: 1.006, SpotUsdPerHourFactor: 0},
			capacityType: "spot",
			expected:     1.006 * billing.SpotDefaultFactor,
		},
		{
			name:         "Spot_negative_factor_falls_back_to_default",
			price:        billing.InstancePrice{OnDemandUsdPerHour: 1.006, SpotUsdPerHourFactor: -0.1},
			capacityType: "spot",
			expected:     1.006 * billing.SpotDefaultFactor,
		},
		{
			name:         "Spot_tiny_positive_factor_overrides_default",
			price:        billing.InstancePrice{OnDemandUsdPerHour: 1.006, SpotUsdPerHourFactor: 0.0001},
			capacityType: "spot",
			expected:     1.006 * 0.0001,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.price.EffectiveUsdPerHour(c.capacityType)
			assert.InDelta(t, c.expected, got, 1e-9)
		})
	}
}

// TestSpotDefaultFactor pins the default factor at the locked 0.30
// value. Catches accidental tuning during refactors.
func TestSpotDefaultFactor(t *testing.T) {
	assert.InDelta(t, 0.30, billing.SpotDefaultFactor, 1e-9, "SpotDefaultFactor is locked at 0.30 for 3.24.6")
}

// TestPriceTable_NewEntries_Pricing spot-checks specific rows added in the
// 2026-04-29 expansion to guard against transcription errors. Each row's
// OnDemandUsdPerHour matches the AWS public pricing JSON for us-east-1.
func TestPriceTable_NewEntries_Pricing(t *testing.T) {
	cases := []struct {
		instanceType string
		expectedUsd  float64
		expectedVcpu int
		expectedMem  float64
	}{
		{"t3.medium", 0.0416, 2, 4},
		{"t3a.medium", 0.0376, 2, 4},
		{"t2.medium", 0.0464, 2, 4},
		{"m4.large", 0.10, 2, 8},
		{"c4.large", 0.10, 2, 3.75},
		{"r4.large", 0.133, 2, 15.25},
	}
	for _, c := range cases {
		t.Run(c.instanceType, func(t *testing.T) {
			p, ok := billing.PriceForInstance(c.instanceType)
			require.True(t, ok, "expected %s in pricing table", c.instanceType)
			assert.InDelta(t, c.expectedUsd, p.OnDemandUsdPerHour, 1e-9)
			assert.Equal(t, c.expectedVcpu, p.VcpuCount)
			assert.InDelta(t, c.expectedMem, p.MemGb, 1e-9)
			assert.Equal(t, 0, p.GpuCount, "%s is CPU-only", c.instanceType)
			assert.Equal(t, "", p.GpuType, "%s has no GPU type", c.instanceType)
		})
	}
}

// TestPriceTable_StructLiteral_Compatibility guards against keyed-literal
// transcription errors that accidentally zero the on-demand rate. Every
// entry must have OnDemandUsdPerHour > 0; a zero value is almost certainly
// a struct-shift bug, not a real price.
func TestPriceTable_StructLiteral_Compatibility(t *testing.T) {
	for k, v := range billing.InstancePricing {
		assert.Greater(t, v.OnDemandUsdPerHour, 0.0, "entry %s must have OnDemandUsdPerHour > 0", k)
		assert.Greater(t, v.VcpuCount, 0, "entry %s must have VcpuCount > 0", k)
		assert.Greater(t, v.MemGb, 0.0, "entry %s must have MemGb > 0", k)
	}
}

// TestPriceTable_OnDemandConsistency_Increasing asserts within-family
// price monotonicity by vCPU count. Catches the swap-bug regression class
// where two entries' OnDemand values are inadvertently transposed (e.g.,
// t3.medium=$0.0832 and t3.large=$0.0416 — both > 0 so the StructLiteral
// guard would pass, but within-family ordering violates the natural
// invariant).
//
// Bucketing: entries are grouped by (family, gpuCount) so families with
// mixed-GPU SKUs (e.g. g4dn.12xlarge has 4 GPUs at $3.912 while
// g4dn.16xlarge has 1 GPU at $4.352) don't false-flag the monotonicity
// check. Within each (family, gpuCount) bucket, higher VcpuCount must
// correspond to higher OnDemandUsdPerHour.
func TestPriceTable_OnDemandConsistency_Increasing(t *testing.T) {
	type entry struct {
		key   string
		price billing.InstancePrice
	}
	// bucketKey is "family|gpuCount" so g4dn-with-1-GPU and g4dn-with-4-GPU
	// are checked separately.
	buckets := map[string][]entry{}

	for k, v := range billing.InstancePricing {
		dot := strings.IndexByte(k, '.')
		if dot <= 0 {
			continue
		}
		fam := k[:dot]
		bucketKey := fmt.Sprintf("%s|gpu=%d", fam, v.GpuCount)
		buckets[bucketKey] = append(buckets[bucketKey], entry{key: k, price: v})
	}

	for bucketKey, entries := range buckets {
		t.Run(bucketKey, func(t *testing.T) {
			if len(entries) < 2 {
				return
			}
			// Insertion sort by VcpuCount ascending.
			for i := 1; i < len(entries); i++ {
				for j := i; j > 0 && entries[j-1].price.VcpuCount > entries[j].price.VcpuCount; j-- {
					entries[j-1], entries[j] = entries[j], entries[j-1]
				}
			}
			for i := 1; i < len(entries); i++ {
				prev := entries[i-1]
				cur := entries[i]
				if prev.price.VcpuCount == cur.price.VcpuCount {
					continue
				}
				assert.LessOrEqual(t, prev.price.OnDemandUsdPerHour, cur.price.OnDemandUsdPerHour,
					"%s ($%.4f, %d vCPU) priced higher than %s ($%.4f, %d vCPU) within %s",
					prev.key, prev.price.OnDemandUsdPerHour, prev.price.VcpuCount,
					cur.key, cur.price.OnDemandUsdPerHour, cur.price.VcpuCount,
					bucketKey)
			}
		})
	}
}
