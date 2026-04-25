// Package billing embeds a static EC2 on-demand price table for rack-side
// cost attribution. Refreshed quarterly via `make refresh-prices`. The
// comment header below is the age gate — the CI drift check in
// prices_test.go warns > 120 days and fails > 180 days.
//
// Source: https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/us-east-1/index.json
// Region: us-east-1 (multi-region is v1.1).
// Last refreshed: 2026-04-22
package billing

// InstancePrice is the on-demand us-east-1 hourly rate plus hardware
// attributes needed by the dominant-resource attribution formula.
type InstancePrice struct {
	OnDemandUsdPerHour float64
	GpuCount           int
	GpuType            string
	VcpuCount          int
	MemGb              float64
}

const pricingTableVersion = "2026-04-22"

// InstancePricing is the canonical static table consumed by the rack's
// budget accumulator. Keyed by EC2 instance type.
var InstancePricing = map[string]InstancePrice{
	// GPU — g4dn (T4)
	"g4dn.xlarge":   {0.526, 1, "T4", 4, 16},
	"g4dn.2xlarge":  {0.752, 1, "T4", 8, 32},
	"g4dn.4xlarge":  {1.204, 1, "T4", 16, 64},
	"g4dn.8xlarge":  {2.176, 1, "T4", 32, 128},
	"g4dn.12xlarge": {3.912, 4, "T4", 48, 192},
	"g4dn.16xlarge": {4.352, 1, "T4", 64, 256},
	"g4dn.metal":    {7.824, 8, "T4", 96, 384},

	// GPU — g5 (A10G)
	"g5.xlarge":   {1.006, 1, "A10G", 4, 16},
	"g5.2xlarge":  {1.212, 1, "A10G", 8, 32},
	"g5.4xlarge":  {1.624, 1, "A10G", 16, 64},
	"g5.8xlarge":  {2.448, 1, "A10G", 32, 128},
	"g5.12xlarge": {5.672, 4, "A10G", 48, 192},
	"g5.16xlarge": {4.096, 1, "A10G", 64, 256},
	"g5.24xlarge": {8.144, 4, "A10G", 96, 384},
	"g5.48xlarge": {16.288, 8, "A10G", 192, 768},

	// GPU — g6 (L4)
	"g6.xlarge":   {0.8048, 1, "L4", 4, 16},
	"g6.2xlarge":  {0.9776, 1, "L4", 8, 32},
	"g6.4xlarge":  {1.3232, 1, "L4", 16, 64},
	"g6.8xlarge":  {2.0144, 1, "L4", 32, 128},
	"g6.12xlarge": {4.6016, 4, "L4", 48, 192},
	"g6.16xlarge": {3.3968, 1, "L4", 64, 256},
	"g6.24xlarge": {6.6752, 4, "L4", 96, 384},
	"g6.48xlarge": {13.3504, 8, "L4", 192, 768},

	// GPU — p3 (V100)
	"p3.2xlarge":  {3.06, 1, "V100", 8, 61},
	"p3.8xlarge":  {12.24, 4, "V100", 32, 244},
	"p3.16xlarge": {24.48, 8, "V100", 64, 488},

	// GPU — p4d (A100)
	"p4d.24xlarge": {32.7726, 8, "A100", 96, 1152},

	// GPU — p5 (H100)
	"p5.48xlarge": {98.32, 8, "H100", 192, 2048},

	// Neuron — inf1 (Inferentia1)
	"inf1.xlarge":   {0.228, 1, "Inferentia1", 4, 8},
	"inf1.2xlarge":  {0.362, 1, "Inferentia1", 8, 16},
	"inf1.6xlarge":  {1.180, 4, "Inferentia1", 24, 48},
	"inf1.24xlarge": {4.721, 16, "Inferentia1", 96, 192},

	// Neuron — inf2 (Inferentia2)
	"inf2.xlarge":   {0.758, 1, "Inferentia2", 4, 16},
	"inf2.8xlarge":  {1.9672, 1, "Inferentia2", 32, 128},
	"inf2.24xlarge": {6.4906, 6, "Inferentia2", 96, 384},
	"inf2.48xlarge": {12.9813, 12, "Inferentia2", 192, 768},

	// Neuron — trn1 (Trainium1)
	"trn1.2xlarge":  {1.3438, 1, "Trainium1", 8, 32},
	"trn1.32xlarge": {21.50, 16, "Trainium1", 128, 512},

	// CPU general — m5
	"m5.large":    {0.096, 0, "", 2, 8},
	"m5.xlarge":   {0.192, 0, "", 4, 16},
	"m5.2xlarge":  {0.384, 0, "", 8, 32},
	"m5.4xlarge":  {0.768, 0, "", 16, 64},
	"m5.8xlarge":  {1.536, 0, "", 32, 128},
	"m5.12xlarge": {2.304, 0, "", 48, 192},
	"m5.16xlarge": {3.072, 0, "", 64, 256},
	"m5.24xlarge": {4.608, 0, "", 96, 384},

	// CPU general — m5a
	"m5a.large":    {0.086, 0, "", 2, 8},
	"m5a.xlarge":   {0.172, 0, "", 4, 16},
	"m5a.2xlarge":  {0.344, 0, "", 8, 32},
	"m5a.4xlarge":  {0.688, 0, "", 16, 64},
	"m5a.8xlarge":  {1.376, 0, "", 32, 128},
	"m5a.12xlarge": {2.064, 0, "", 48, 192},
	"m5a.16xlarge": {2.752, 0, "", 64, 256},
	"m5a.24xlarge": {4.128, 0, "", 96, 384},

	// CPU general — m6i
	"m6i.large":    {0.096, 0, "", 2, 8},
	"m6i.xlarge":   {0.192, 0, "", 4, 16},
	"m6i.2xlarge":  {0.384, 0, "", 8, 32},
	"m6i.4xlarge":  {0.768, 0, "", 16, 64},
	"m6i.8xlarge":  {1.536, 0, "", 32, 128},
	"m6i.12xlarge": {2.304, 0, "", 48, 192},
	"m6i.16xlarge": {3.072, 0, "", 64, 256},
	"m6i.24xlarge": {4.608, 0, "", 96, 384},
	"m6i.32xlarge": {6.144, 0, "", 128, 512},

	// CPU general — m6a
	"m6a.large":    {0.0864, 0, "", 2, 8},
	"m6a.xlarge":   {0.1728, 0, "", 4, 16},
	"m6a.2xlarge":  {0.3456, 0, "", 8, 32},
	"m6a.4xlarge":  {0.6912, 0, "", 16, 64},
	"m6a.8xlarge":  {1.3824, 0, "", 32, 128},
	"m6a.12xlarge": {2.0736, 0, "", 48, 192},
	"m6a.16xlarge": {2.7648, 0, "", 64, 256},
	"m6a.24xlarge": {4.1472, 0, "", 96, 384},
	"m6a.32xlarge": {5.5296, 0, "", 128, 512},
	"m6a.48xlarge": {8.2944, 0, "", 192, 768},

	// CPU general — m7i
	"m7i.large":    {0.1008, 0, "", 2, 8},
	"m7i.xlarge":   {0.2016, 0, "", 4, 16},
	"m7i.2xlarge":  {0.4032, 0, "", 8, 32},
	"m7i.4xlarge":  {0.8064, 0, "", 16, 64},
	"m7i.8xlarge":  {1.6128, 0, "", 32, 128},
	"m7i.12xlarge": {2.4192, 0, "", 48, 192},
	"m7i.16xlarge": {3.2256, 0, "", 64, 256},
	"m7i.24xlarge": {4.8384, 0, "", 96, 384},
	"m7i.48xlarge": {9.6768, 0, "", 192, 768},

	// CPU compute — c5
	"c5.large":    {0.085, 0, "", 2, 4},
	"c5.xlarge":   {0.170, 0, "", 4, 8},
	"c5.2xlarge":  {0.340, 0, "", 8, 16},
	"c5.4xlarge":  {0.680, 0, "", 16, 32},
	"c5.9xlarge":  {1.530, 0, "", 36, 72},
	"c5.12xlarge": {2.040, 0, "", 48, 96},
	"c5.18xlarge": {3.060, 0, "", 72, 144},
	"c5.24xlarge": {4.080, 0, "", 96, 192},

	// CPU compute — c6i
	"c6i.large":    {0.085, 0, "", 2, 4},
	"c6i.xlarge":   {0.170, 0, "", 4, 8},
	"c6i.2xlarge":  {0.340, 0, "", 8, 16},
	"c6i.4xlarge":  {0.680, 0, "", 16, 32},
	"c6i.8xlarge":  {1.360, 0, "", 32, 64},
	"c6i.12xlarge": {2.040, 0, "", 48, 96},
	"c6i.16xlarge": {2.720, 0, "", 64, 128},
	"c6i.24xlarge": {4.080, 0, "", 96, 192},
	"c6i.32xlarge": {5.440, 0, "", 128, 256},

	// CPU compute — c7i
	"c7i.large":    {0.0893, 0, "", 2, 4},
	"c7i.xlarge":   {0.1785, 0, "", 4, 8},
	"c7i.2xlarge":  {0.357, 0, "", 8, 16},
	"c7i.4xlarge":  {0.714, 0, "", 16, 32},
	"c7i.8xlarge":  {1.428, 0, "", 32, 64},
	"c7i.12xlarge": {2.142, 0, "", 48, 96},
	"c7i.16xlarge": {2.856, 0, "", 64, 128},
	"c7i.24xlarge": {4.284, 0, "", 96, 192},
	"c7i.48xlarge": {8.568, 0, "", 192, 384},

	// CPU memory — r5
	"r5.large":    {0.126, 0, "", 2, 16},
	"r5.xlarge":   {0.252, 0, "", 4, 32},
	"r5.2xlarge":  {0.504, 0, "", 8, 64},
	"r5.4xlarge":  {1.008, 0, "", 16, 128},
	"r5.8xlarge":  {2.016, 0, "", 32, 256},
	"r5.12xlarge": {3.024, 0, "", 48, 384},
	"r5.16xlarge": {4.032, 0, "", 64, 512},
	"r5.24xlarge": {6.048, 0, "", 96, 768},

	// CPU memory — r6i
	"r6i.large":    {0.126, 0, "", 2, 16},
	"r6i.xlarge":   {0.252, 0, "", 4, 32},
	"r6i.2xlarge":  {0.504, 0, "", 8, 64},
	"r6i.4xlarge":  {1.008, 0, "", 16, 128},
	"r6i.8xlarge":  {2.016, 0, "", 32, 256},
	"r6i.12xlarge": {3.024, 0, "", 48, 384},
	"r6i.16xlarge": {4.032, 0, "", 64, 512},
	"r6i.24xlarge": {6.048, 0, "", 96, 768},
	"r6i.32xlarge": {8.064, 0, "", 128, 1024},

	// CPU memory — r7i
	"r7i.large":    {0.1323, 0, "", 2, 16},
	"r7i.xlarge":   {0.2646, 0, "", 4, 32},
	"r7i.2xlarge":  {0.5292, 0, "", 8, 64},
	"r7i.4xlarge":  {1.0584, 0, "", 16, 128},
	"r7i.8xlarge":  {2.1168, 0, "", 32, 256},
	"r7i.12xlarge": {3.1752, 0, "", 48, 384},
	"r7i.16xlarge": {4.2336, 0, "", 64, 512},
	"r7i.24xlarge": {6.3504, 0, "", 96, 768},
	"r7i.48xlarge": {12.7008, 0, "", 192, 1536},
}

// PriceForInstance looks up an instance type and returns (price, true) on hit,
// (zero-value, false) otherwise. Callers should increment WarningCount and
// skip cost attribution on false rather than erroring — keeps the accumulator
// robust to instance families introduced between quarterly refreshes.
func PriceForInstance(instanceType string) (InstancePrice, bool) {
	p, ok := InstancePricing[instanceType]
	return p, ok
}

// PricingTableVersion returns the last refresh date string from the header
// comment. Used to populate AppCost.PricingTableVersion.
func PricingTableVersion() string {
	return pricingTableVersion
}
