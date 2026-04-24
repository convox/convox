package billing_test

import (
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
// upper-bound canary ($500/hr) or below the lower-bound canary ($0.01/hr)
// is almost certainly a parser bug, not a real AWS rate.
func TestPriceTable_SanityFloors(t *testing.T) {
	const (
		upperCanary = 500.0
		lowerCanary = 0.01
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
		t.Logf("WARNING: price table is %s old — run `make refresh-prices` (soft warn > 120 days)", age)
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
