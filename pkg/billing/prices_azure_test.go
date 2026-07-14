package billing_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/billing"
	"github.com/stretchr/testify/assert"
)

// mirrors prices_test.go guards for the Azure table
func TestAzurePriceTable_SanityFloors(t *testing.T) {
	for k, v := range billing.AzureInstancePricing {
		assert.Less(t, v.OnDemandUsdPerHour, 500.0, "price for %s exceeds sanity ceiling", k)
		assert.GreaterOrEqual(t, v.OnDemandUsdPerHour, 0.001, "price for %s below sanity floor", k)
	}
}

func TestAzurePriceTable_KeyFormat(t *testing.T) {
	re := regexp.MustCompile(`^Standard_[A-Za-z0-9_]+$`)
	for k := range billing.AzureInstancePricing {
		assert.True(t, re.MatchString(k), "unexpected key shape: %s", k)
		assert.NotContains(t, k, "-", "constrained-vCPU SKU leaked: %s", k)
	}
}

func TestAzurePriceTable_EntryMetadata(t *testing.T) {
	gpuTypes := map[string]bool{"T4": true, "A100": true, "H100": true, "A10": true}
	for k, v := range billing.AzureInstancePricing {
		assert.Greater(t, v.VcpuCount, 0, "%s missing vcpu", k)
		assert.Greater(t, v.MemGb, 0.0, "%s missing memory", k)
		gpuFamily := strings.HasPrefix(k, "Standard_NC") || strings.HasPrefix(k, "Standard_ND") || strings.HasPrefix(k, "Standard_NV")
		if gpuFamily || v.GpuCount > 0 || v.GpuType != "" {
			assert.GreaterOrEqual(t, v.GpuCount, 1, "%s gpu count", k)
			assert.True(t, gpuTypes[v.GpuType], "%s gpu type %q outside vocabulary", k, v.GpuType)
		}
		if v.SpotUsdPerHourFactor != 0 {
			assert.Greater(t, v.SpotUsdPerHourFactor, 0.0, "%s spot factor", k)
			assert.Less(t, v.SpotUsdPerHourFactor, 1.0, "%s spot factor", k)
		}
	}
}

func TestAzurePriceTable_FamilyCoverage(t *testing.T) {
	families := map[string]string{
		"B":      `^Standard_B\d`,
		"Dv2":    `^Standard_DS?\d+_v2$`,
		"Dv3":    `^Standard_D\d+s?_v3$`,
		"Dv4":    `^Standard_D\d+d?s?_v4$`,
		"Dv5":    `^Standard_D\d+d?s?_v5$`,
		"Dsv6":   `^Standard_D\d+s_v6$`,
		"Das":    `^Standard_D\d+as_v[456]$`,
		"Dads":   `^Standard_D\d+ads_v[56]$`,
		"Dps":    `^Standard_D\d+pd?s_v[56]$`,
		"Fsv2":   `^Standard_F\d+s_v2$`,
		"Fasv6":  `^Standard_F\d+as_v6$`,
		"Ev3":    `^Standard_E\d+s?_v3$`,
		"Ev4":    `^Standard_E\d+i?d?s?_v4$`,
		"Easv4":  `^Standard_E\d+as_v4$`,
		"Ev5":    `^Standard_E\d+i?d?s?_v5$`,
		"Easv56": `^Standard_E\d+i?ad?s_v[56]$`,
		"Esv6":   `^Standard_E\d+i?s_v6$`,
		"M":      `^Standard_M\d`,
		"NCasT4": `^Standard_NC\d+as_T4_v3$`,
		"NCA100": `^Standard_NC\d+ads_A100_v4$`,
		"NDA100": `^Standard_ND96(asr_v4|ams_A100_v4|amsr_A100_v4)$`,
		"NCH100": `^Standard_NC(40ads|80adis)_H100_v5$`,
		"NDH100": `^Standard_ND96isr_H100_v5$`,
		"NVA10":  `^Standard_NV\d+adm?s_A10_v5$`,
	}
	for fam, pat := range families {
		t.Run(fam, func(t *testing.T) {
			re := regexp.MustCompile(pat)
			found := false
			for k := range billing.AzureInstancePricing {
				if re.MatchString(k) {
					found = true
					break
				}
			}
			assert.True(t, found, "missing price entries for family %s", fam)
		})
	}
}
