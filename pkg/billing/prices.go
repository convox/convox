package billing

type InstancePrice struct {
	OnDemandUsdPerHour   float64
	SpotUsdPerHourFactor float64
	GpuCount             int
	GpuType              string
	VcpuCount            int
	MemGb                float64
}

// SpotDefaultFactor approximates the ~70% spot discount across stable families.
const SpotDefaultFactor = 0.30

const pricingTableVersion = "2026-04-29"

const PricingSourceStaticTable = "on-demand-static-table"

var InstancePricing = map[string]InstancePrice{
	// GPU — g4dn (T4)
	"g4dn.xlarge":   {OnDemandUsdPerHour: 0.526, GpuCount: 1, GpuType: "T4", VcpuCount: 4, MemGb: 16},
	"g4dn.2xlarge":  {OnDemandUsdPerHour: 0.752, GpuCount: 1, GpuType: "T4", VcpuCount: 8, MemGb: 32},
	"g4dn.4xlarge":  {OnDemandUsdPerHour: 1.204, GpuCount: 1, GpuType: "T4", VcpuCount: 16, MemGb: 64},
	"g4dn.8xlarge":  {OnDemandUsdPerHour: 2.176, GpuCount: 1, GpuType: "T4", VcpuCount: 32, MemGb: 128},
	"g4dn.12xlarge": {OnDemandUsdPerHour: 3.912, GpuCount: 4, GpuType: "T4", VcpuCount: 48, MemGb: 192},
	"g4dn.16xlarge": {OnDemandUsdPerHour: 4.352, GpuCount: 1, GpuType: "T4", VcpuCount: 64, MemGb: 256},
	"g4dn.metal":    {OnDemandUsdPerHour: 7.824, GpuCount: 8, GpuType: "T4", VcpuCount: 96, MemGb: 384},

	// GPU — g5 (A10G)
	"g5.xlarge":   {OnDemandUsdPerHour: 1.006, GpuCount: 1, GpuType: "A10G", VcpuCount: 4, MemGb: 16},
	"g5.2xlarge":  {OnDemandUsdPerHour: 1.212, GpuCount: 1, GpuType: "A10G", VcpuCount: 8, MemGb: 32},
	"g5.4xlarge":  {OnDemandUsdPerHour: 1.624, GpuCount: 1, GpuType: "A10G", VcpuCount: 16, MemGb: 64},
	"g5.8xlarge":  {OnDemandUsdPerHour: 2.448, GpuCount: 1, GpuType: "A10G", VcpuCount: 32, MemGb: 128},
	"g5.12xlarge": {OnDemandUsdPerHour: 5.672, GpuCount: 4, GpuType: "A10G", VcpuCount: 48, MemGb: 192},
	"g5.16xlarge": {OnDemandUsdPerHour: 4.096, GpuCount: 1, GpuType: "A10G", VcpuCount: 64, MemGb: 256},
	"g5.24xlarge": {OnDemandUsdPerHour: 8.144, GpuCount: 4, GpuType: "A10G", VcpuCount: 96, MemGb: 384},
	"g5.48xlarge": {OnDemandUsdPerHour: 16.288, GpuCount: 8, GpuType: "A10G", VcpuCount: 192, MemGb: 768},

	// GPU — g6 (L4)
	"g6.xlarge":   {OnDemandUsdPerHour: 0.8048, GpuCount: 1, GpuType: "L4", VcpuCount: 4, MemGb: 16},
	"g6.2xlarge":  {OnDemandUsdPerHour: 0.9776, GpuCount: 1, GpuType: "L4", VcpuCount: 8, MemGb: 32},
	"g6.4xlarge":  {OnDemandUsdPerHour: 1.3232, GpuCount: 1, GpuType: "L4", VcpuCount: 16, MemGb: 64},
	"g6.8xlarge":  {OnDemandUsdPerHour: 2.0144, GpuCount: 1, GpuType: "L4", VcpuCount: 32, MemGb: 128},
	"g6.12xlarge": {OnDemandUsdPerHour: 4.6016, GpuCount: 4, GpuType: "L4", VcpuCount: 48, MemGb: 192},
	"g6.16xlarge": {OnDemandUsdPerHour: 3.3968, GpuCount: 1, GpuType: "L4", VcpuCount: 64, MemGb: 256},
	"g6.24xlarge": {OnDemandUsdPerHour: 6.6752, GpuCount: 4, GpuType: "L4", VcpuCount: 96, MemGb: 384},
	"g6.48xlarge": {OnDemandUsdPerHour: 13.3504, GpuCount: 8, GpuType: "L4", VcpuCount: 192, MemGb: 768},

	// GPU — p3 (V100)
	"p3.2xlarge":  {OnDemandUsdPerHour: 3.06, GpuCount: 1, GpuType: "V100", VcpuCount: 8, MemGb: 61},
	"p3.8xlarge":  {OnDemandUsdPerHour: 12.24, GpuCount: 4, GpuType: "V100", VcpuCount: 32, MemGb: 244},
	"p3.16xlarge": {OnDemandUsdPerHour: 24.48, GpuCount: 8, GpuType: "V100", VcpuCount: 64, MemGb: 488},

	// GPU — p4d (A100)
	"p4d.24xlarge": {OnDemandUsdPerHour: 32.7726, GpuCount: 8, GpuType: "A100", VcpuCount: 96, MemGb: 1152},

	// GPU — p5 (H100)
	"p5.48xlarge": {OnDemandUsdPerHour: 98.32, GpuCount: 8, GpuType: "H100", VcpuCount: 192, MemGb: 2048},

	// Neuron — inf1 (Inferentia1)
	"inf1.xlarge":   {OnDemandUsdPerHour: 0.228, GpuCount: 1, GpuType: "Inferentia1", VcpuCount: 4, MemGb: 8},
	"inf1.2xlarge":  {OnDemandUsdPerHour: 0.362, GpuCount: 1, GpuType: "Inferentia1", VcpuCount: 8, MemGb: 16},
	"inf1.6xlarge":  {OnDemandUsdPerHour: 1.180, GpuCount: 4, GpuType: "Inferentia1", VcpuCount: 24, MemGb: 48},
	"inf1.24xlarge": {OnDemandUsdPerHour: 4.721, GpuCount: 16, GpuType: "Inferentia1", VcpuCount: 96, MemGb: 192},

	// Neuron — inf2 (Inferentia2)
	"inf2.xlarge":   {OnDemandUsdPerHour: 0.758, GpuCount: 1, GpuType: "Inferentia2", VcpuCount: 4, MemGb: 16},
	"inf2.8xlarge":  {OnDemandUsdPerHour: 1.9672, GpuCount: 1, GpuType: "Inferentia2", VcpuCount: 32, MemGb: 128},
	"inf2.24xlarge": {OnDemandUsdPerHour: 6.4906, GpuCount: 6, GpuType: "Inferentia2", VcpuCount: 96, MemGb: 384},
	"inf2.48xlarge": {OnDemandUsdPerHour: 12.9813, GpuCount: 12, GpuType: "Inferentia2", VcpuCount: 192, MemGb: 768},

	// Neuron — trn1 (Trainium1)
	"trn1.2xlarge":  {OnDemandUsdPerHour: 1.3438, GpuCount: 1, GpuType: "Trainium1", VcpuCount: 8, MemGb: 32},
	"trn1.32xlarge": {OnDemandUsdPerHour: 21.50, GpuCount: 16, GpuType: "Trainium1", VcpuCount: 128, MemGb: 512},

	// CPU general — m5
	"m5.large":    {OnDemandUsdPerHour: 0.096, VcpuCount: 2, MemGb: 8},
	"m5.xlarge":   {OnDemandUsdPerHour: 0.192, VcpuCount: 4, MemGb: 16},
	"m5.2xlarge":  {OnDemandUsdPerHour: 0.384, VcpuCount: 8, MemGb: 32},
	"m5.4xlarge":  {OnDemandUsdPerHour: 0.768, VcpuCount: 16, MemGb: 64},
	"m5.8xlarge":  {OnDemandUsdPerHour: 1.536, VcpuCount: 32, MemGb: 128},
	"m5.12xlarge": {OnDemandUsdPerHour: 2.304, VcpuCount: 48, MemGb: 192},
	"m5.16xlarge": {OnDemandUsdPerHour: 3.072, VcpuCount: 64, MemGb: 256},
	"m5.24xlarge": {OnDemandUsdPerHour: 4.608, VcpuCount: 96, MemGb: 384},

	// CPU general — m5a
	"m5a.large":    {OnDemandUsdPerHour: 0.086, VcpuCount: 2, MemGb: 8},
	"m5a.xlarge":   {OnDemandUsdPerHour: 0.172, VcpuCount: 4, MemGb: 16},
	"m5a.2xlarge":  {OnDemandUsdPerHour: 0.344, VcpuCount: 8, MemGb: 32},
	"m5a.4xlarge":  {OnDemandUsdPerHour: 0.688, VcpuCount: 16, MemGb: 64},
	"m5a.8xlarge":  {OnDemandUsdPerHour: 1.376, VcpuCount: 32, MemGb: 128},
	"m5a.12xlarge": {OnDemandUsdPerHour: 2.064, VcpuCount: 48, MemGb: 192},
	"m5a.16xlarge": {OnDemandUsdPerHour: 2.752, VcpuCount: 64, MemGb: 256},
	"m5a.24xlarge": {OnDemandUsdPerHour: 4.128, VcpuCount: 96, MemGb: 384},

	// CPU general — m6i
	"m6i.large":    {OnDemandUsdPerHour: 0.096, VcpuCount: 2, MemGb: 8},
	"m6i.xlarge":   {OnDemandUsdPerHour: 0.192, VcpuCount: 4, MemGb: 16},
	"m6i.2xlarge":  {OnDemandUsdPerHour: 0.384, VcpuCount: 8, MemGb: 32},
	"m6i.4xlarge":  {OnDemandUsdPerHour: 0.768, VcpuCount: 16, MemGb: 64},
	"m6i.8xlarge":  {OnDemandUsdPerHour: 1.536, VcpuCount: 32, MemGb: 128},
	"m6i.12xlarge": {OnDemandUsdPerHour: 2.304, VcpuCount: 48, MemGb: 192},
	"m6i.16xlarge": {OnDemandUsdPerHour: 3.072, VcpuCount: 64, MemGb: 256},
	"m6i.24xlarge": {OnDemandUsdPerHour: 4.608, VcpuCount: 96, MemGb: 384},
	"m6i.32xlarge": {OnDemandUsdPerHour: 6.144, VcpuCount: 128, MemGb: 512},

	// CPU general — m6a
	"m6a.large":    {OnDemandUsdPerHour: 0.0864, VcpuCount: 2, MemGb: 8},
	"m6a.xlarge":   {OnDemandUsdPerHour: 0.1728, VcpuCount: 4, MemGb: 16},
	"m6a.2xlarge":  {OnDemandUsdPerHour: 0.3456, VcpuCount: 8, MemGb: 32},
	"m6a.4xlarge":  {OnDemandUsdPerHour: 0.6912, VcpuCount: 16, MemGb: 64},
	"m6a.8xlarge":  {OnDemandUsdPerHour: 1.3824, VcpuCount: 32, MemGb: 128},
	"m6a.12xlarge": {OnDemandUsdPerHour: 2.0736, VcpuCount: 48, MemGb: 192},
	"m6a.16xlarge": {OnDemandUsdPerHour: 2.7648, VcpuCount: 64, MemGb: 256},
	"m6a.24xlarge": {OnDemandUsdPerHour: 4.1472, VcpuCount: 96, MemGb: 384},
	"m6a.32xlarge": {OnDemandUsdPerHour: 5.5296, VcpuCount: 128, MemGb: 512},
	"m6a.48xlarge": {OnDemandUsdPerHour: 8.2944, VcpuCount: 192, MemGb: 768},

	// CPU general — m7i
	"m7i.large":    {OnDemandUsdPerHour: 0.1008, VcpuCount: 2, MemGb: 8},
	"m7i.xlarge":   {OnDemandUsdPerHour: 0.2016, VcpuCount: 4, MemGb: 16},
	"m7i.2xlarge":  {OnDemandUsdPerHour: 0.4032, VcpuCount: 8, MemGb: 32},
	"m7i.4xlarge":  {OnDemandUsdPerHour: 0.8064, VcpuCount: 16, MemGb: 64},
	"m7i.8xlarge":  {OnDemandUsdPerHour: 1.6128, VcpuCount: 32, MemGb: 128},
	"m7i.12xlarge": {OnDemandUsdPerHour: 2.4192, VcpuCount: 48, MemGb: 192},
	"m7i.16xlarge": {OnDemandUsdPerHour: 3.2256, VcpuCount: 64, MemGb: 256},
	"m7i.24xlarge": {OnDemandUsdPerHour: 4.8384, VcpuCount: 96, MemGb: 384},
	"m7i.48xlarge": {OnDemandUsdPerHour: 9.6768, VcpuCount: 192, MemGb: 768},

	// CPU compute — c5
	"c5.large":    {OnDemandUsdPerHour: 0.085, VcpuCount: 2, MemGb: 4},
	"c5.xlarge":   {OnDemandUsdPerHour: 0.170, VcpuCount: 4, MemGb: 8},
	"c5.2xlarge":  {OnDemandUsdPerHour: 0.340, VcpuCount: 8, MemGb: 16},
	"c5.4xlarge":  {OnDemandUsdPerHour: 0.680, VcpuCount: 16, MemGb: 32},
	"c5.9xlarge":  {OnDemandUsdPerHour: 1.530, VcpuCount: 36, MemGb: 72},
	"c5.12xlarge": {OnDemandUsdPerHour: 2.040, VcpuCount: 48, MemGb: 96},
	"c5.18xlarge": {OnDemandUsdPerHour: 3.060, VcpuCount: 72, MemGb: 144},
	"c5.24xlarge": {OnDemandUsdPerHour: 4.080, VcpuCount: 96, MemGb: 192},

	// CPU compute — c6i
	"c6i.large":    {OnDemandUsdPerHour: 0.085, VcpuCount: 2, MemGb: 4},
	"c6i.xlarge":   {OnDemandUsdPerHour: 0.170, VcpuCount: 4, MemGb: 8},
	"c6i.2xlarge":  {OnDemandUsdPerHour: 0.340, VcpuCount: 8, MemGb: 16},
	"c6i.4xlarge":  {OnDemandUsdPerHour: 0.680, VcpuCount: 16, MemGb: 32},
	"c6i.8xlarge":  {OnDemandUsdPerHour: 1.360, VcpuCount: 32, MemGb: 64},
	"c6i.12xlarge": {OnDemandUsdPerHour: 2.040, VcpuCount: 48, MemGb: 96},
	"c6i.16xlarge": {OnDemandUsdPerHour: 2.720, VcpuCount: 64, MemGb: 128},
	"c6i.24xlarge": {OnDemandUsdPerHour: 4.080, VcpuCount: 96, MemGb: 192},
	"c6i.32xlarge": {OnDemandUsdPerHour: 5.440, VcpuCount: 128, MemGb: 256},

	// CPU compute — c7i
	"c7i.large":    {OnDemandUsdPerHour: 0.0893, VcpuCount: 2, MemGb: 4},
	"c7i.xlarge":   {OnDemandUsdPerHour: 0.1785, VcpuCount: 4, MemGb: 8},
	"c7i.2xlarge":  {OnDemandUsdPerHour: 0.357, VcpuCount: 8, MemGb: 16},
	"c7i.4xlarge":  {OnDemandUsdPerHour: 0.714, VcpuCount: 16, MemGb: 32},
	"c7i.8xlarge":  {OnDemandUsdPerHour: 1.428, VcpuCount: 32, MemGb: 64},
	"c7i.12xlarge": {OnDemandUsdPerHour: 2.142, VcpuCount: 48, MemGb: 96},
	"c7i.16xlarge": {OnDemandUsdPerHour: 2.856, VcpuCount: 64, MemGb: 128},
	"c7i.24xlarge": {OnDemandUsdPerHour: 4.284, VcpuCount: 96, MemGb: 192},
	"c7i.48xlarge": {OnDemandUsdPerHour: 8.568, VcpuCount: 192, MemGb: 384},

	// CPU memory — r5
	"r5.large":    {OnDemandUsdPerHour: 0.126, VcpuCount: 2, MemGb: 16},
	"r5.xlarge":   {OnDemandUsdPerHour: 0.252, VcpuCount: 4, MemGb: 32},
	"r5.2xlarge":  {OnDemandUsdPerHour: 0.504, VcpuCount: 8, MemGb: 64},
	"r5.4xlarge":  {OnDemandUsdPerHour: 1.008, VcpuCount: 16, MemGb: 128},
	"r5.8xlarge":  {OnDemandUsdPerHour: 2.016, VcpuCount: 32, MemGb: 256},
	"r5.12xlarge": {OnDemandUsdPerHour: 3.024, VcpuCount: 48, MemGb: 384},
	"r5.16xlarge": {OnDemandUsdPerHour: 4.032, VcpuCount: 64, MemGb: 512},
	"r5.24xlarge": {OnDemandUsdPerHour: 6.048, VcpuCount: 96, MemGb: 768},

	// CPU memory — r6i
	"r6i.large":    {OnDemandUsdPerHour: 0.126, VcpuCount: 2, MemGb: 16},
	"r6i.xlarge":   {OnDemandUsdPerHour: 0.252, VcpuCount: 4, MemGb: 32},
	"r6i.2xlarge":  {OnDemandUsdPerHour: 0.504, VcpuCount: 8, MemGb: 64},
	"r6i.4xlarge":  {OnDemandUsdPerHour: 1.008, VcpuCount: 16, MemGb: 128},
	"r6i.8xlarge":  {OnDemandUsdPerHour: 2.016, VcpuCount: 32, MemGb: 256},
	"r6i.12xlarge": {OnDemandUsdPerHour: 3.024, VcpuCount: 48, MemGb: 384},
	"r6i.16xlarge": {OnDemandUsdPerHour: 4.032, VcpuCount: 64, MemGb: 512},
	"r6i.24xlarge": {OnDemandUsdPerHour: 6.048, VcpuCount: 96, MemGb: 768},
	"r6i.32xlarge": {OnDemandUsdPerHour: 8.064, VcpuCount: 128, MemGb: 1024},

	// CPU memory — r7i
	"r7i.large":    {OnDemandUsdPerHour: 0.1323, VcpuCount: 2, MemGb: 16},
	"r7i.xlarge":   {OnDemandUsdPerHour: 0.2646, VcpuCount: 4, MemGb: 32},
	"r7i.2xlarge":  {OnDemandUsdPerHour: 0.5292, VcpuCount: 8, MemGb: 64},
	"r7i.4xlarge":  {OnDemandUsdPerHour: 1.0584, VcpuCount: 16, MemGb: 128},
	"r7i.8xlarge":  {OnDemandUsdPerHour: 2.1168, VcpuCount: 32, MemGb: 256},
	"r7i.12xlarge": {OnDemandUsdPerHour: 3.1752, VcpuCount: 48, MemGb: 384},
	"r7i.16xlarge": {OnDemandUsdPerHour: 4.2336, VcpuCount: 64, MemGb: 512},
	"r7i.24xlarge": {OnDemandUsdPerHour: 6.3504, VcpuCount: 96, MemGb: 768},
	"r7i.48xlarge": {OnDemandUsdPerHour: 12.7008, VcpuCount: 192, MemGb: 1536},

	// CPU general — t2 (legacy burstable, no Nitro)
	"t2.nano":    {OnDemandUsdPerHour: 0.0058, VcpuCount: 1, MemGb: 0.5},
	"t2.micro":   {OnDemandUsdPerHour: 0.0116, VcpuCount: 1, MemGb: 1},
	"t2.small":   {OnDemandUsdPerHour: 0.023, VcpuCount: 1, MemGb: 2},
	"t2.medium":  {OnDemandUsdPerHour: 0.0464, VcpuCount: 2, MemGb: 4},
	"t2.large":   {OnDemandUsdPerHour: 0.0928, VcpuCount: 2, MemGb: 8},
	"t2.xlarge":  {OnDemandUsdPerHour: 0.1856, VcpuCount: 4, MemGb: 16},
	"t2.2xlarge": {OnDemandUsdPerHour: 0.3712, VcpuCount: 8, MemGb: 32},

	// CPU general — t3 (Nitro burstable, AMD64)
	"t3.nano":    {OnDemandUsdPerHour: 0.0052, VcpuCount: 2, MemGb: 0.5},
	"t3.micro":   {OnDemandUsdPerHour: 0.0104, VcpuCount: 2, MemGb: 1},
	"t3.small":   {OnDemandUsdPerHour: 0.0208, VcpuCount: 2, MemGb: 2},
	"t3.medium":  {OnDemandUsdPerHour: 0.0416, VcpuCount: 2, MemGb: 4},
	"t3.large":   {OnDemandUsdPerHour: 0.0832, VcpuCount: 2, MemGb: 8},
	"t3.xlarge":  {OnDemandUsdPerHour: 0.1664, VcpuCount: 4, MemGb: 16},
	"t3.2xlarge": {OnDemandUsdPerHour: 0.3328, VcpuCount: 8, MemGb: 32},

	// CPU general — t3a (AMD-EPYC variant of t3)
	"t3a.nano":    {OnDemandUsdPerHour: 0.0047, VcpuCount: 2, MemGb: 0.5},
	"t3a.micro":   {OnDemandUsdPerHour: 0.0094, VcpuCount: 2, MemGb: 1},
	"t3a.small":   {OnDemandUsdPerHour: 0.0188, VcpuCount: 2, MemGb: 2},
	"t3a.medium":  {OnDemandUsdPerHour: 0.0376, VcpuCount: 2, MemGb: 4},
	"t3a.large":   {OnDemandUsdPerHour: 0.0752, VcpuCount: 2, MemGb: 8},
	"t3a.xlarge":  {OnDemandUsdPerHour: 0.1504, VcpuCount: 4, MemGb: 16},
	"t3a.2xlarge": {OnDemandUsdPerHour: 0.3008, VcpuCount: 8, MemGb: 32},

	// CPU general — m4 (legacy, pre-Nitro)
	"m4.large":    {OnDemandUsdPerHour: 0.10, VcpuCount: 2, MemGb: 8},
	"m4.xlarge":   {OnDemandUsdPerHour: 0.20, VcpuCount: 4, MemGb: 16},
	"m4.2xlarge":  {OnDemandUsdPerHour: 0.40, VcpuCount: 8, MemGb: 32},
	"m4.4xlarge":  {OnDemandUsdPerHour: 0.80, VcpuCount: 16, MemGb: 64},
	"m4.10xlarge": {OnDemandUsdPerHour: 2.00, VcpuCount: 40, MemGb: 160},
	"m4.16xlarge": {OnDemandUsdPerHour: 3.20, VcpuCount: 64, MemGb: 256},

	// CPU compute — c4 (legacy)
	"c4.large":   {OnDemandUsdPerHour: 0.10, VcpuCount: 2, MemGb: 3.75},
	"c4.xlarge":  {OnDemandUsdPerHour: 0.199, VcpuCount: 4, MemGb: 7.5},
	"c4.2xlarge": {OnDemandUsdPerHour: 0.398, VcpuCount: 8, MemGb: 15},
	"c4.4xlarge": {OnDemandUsdPerHour: 0.796, VcpuCount: 16, MemGb: 30},
	"c4.8xlarge": {OnDemandUsdPerHour: 1.591, VcpuCount: 36, MemGb: 60},

	// CPU memory — r4 (legacy)
	"r4.large":    {OnDemandUsdPerHour: 0.133, VcpuCount: 2, MemGb: 15.25},
	"r4.xlarge":   {OnDemandUsdPerHour: 0.266, VcpuCount: 4, MemGb: 30.5},
	"r4.2xlarge":  {OnDemandUsdPerHour: 0.532, VcpuCount: 8, MemGb: 61},
	"r4.4xlarge":  {OnDemandUsdPerHour: 1.064, VcpuCount: 16, MemGb: 122},
	"r4.8xlarge":  {OnDemandUsdPerHour: 2.128, VcpuCount: 32, MemGb: 244},
	"r4.16xlarge": {OnDemandUsdPerHour: 4.256, VcpuCount: 64, MemGb: 488},
}

func PriceForInstance(instanceType string) (InstancePrice, bool) {
	p, ok := InstancePricing[instanceType]
	return p, ok
}

func (p InstancePrice) EffectiveUsdPerHour(capacityType string) float64 {
	if capacityType != "spot" {
		return p.OnDemandUsdPerHour
	}
	factor := p.SpotUsdPerHourFactor
	if factor <= 0 {
		factor = SpotDefaultFactor
	}
	return p.OnDemandUsdPerHour * factor
}

func PricingTableVersion() string {
	return pricingTableVersion
}
