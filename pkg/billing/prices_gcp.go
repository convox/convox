// GCP GPU instance pricing. Hand-maintained (not code-generated): the table
// covers every machine family in provider/gcp gcpGpuMachinePrefixes (kept in
// sync by TestGcpGpuFamiliesArePriced). Prices are approximate us-central1
// on-demand list prices in USD per hour, as of 2026-07-22 from
// cloud.google.com/products/compute/pricing/accelerator-optimized, and are used
// only for relative cost accounting, not billing. GpuType/GpuCount metadata
// cross-checked against cloud.google.com/compute/docs machine families.
// Fractional-GPU G4 sizes (g4-standard-6/12/24) are omitted: GpuCount is an
// integer and those sizes share one physical RTX PRO 6000.

package billing

var GcpInstancePricing = map[string]InstancePrice{
	// GPU: G2 (L4)
	"g2-standard-4":  {OnDemandUsdPerHour: 0.708, GpuCount: 1, GpuType: "L4", VcpuCount: 4, MemGb: 16},
	"g2-standard-8":  {OnDemandUsdPerHour: 0.854, GpuCount: 1, GpuType: "L4", VcpuCount: 8, MemGb: 32},
	"g2-standard-12": {OnDemandUsdPerHour: 1.000, GpuCount: 1, GpuType: "L4", VcpuCount: 12, MemGb: 48},
	"g2-standard-16": {OnDemandUsdPerHour: 1.146, GpuCount: 1, GpuType: "L4", VcpuCount: 16, MemGb: 64},
	"g2-standard-24": {OnDemandUsdPerHour: 2.000, GpuCount: 2, GpuType: "L4", VcpuCount: 24, MemGb: 96},
	"g2-standard-32": {OnDemandUsdPerHour: 1.729, GpuCount: 1, GpuType: "L4", VcpuCount: 32, MemGb: 128},
	"g2-standard-48": {OnDemandUsdPerHour: 4.000, GpuCount: 4, GpuType: "L4", VcpuCount: 48, MemGb: 192},
	"g2-standard-96": {OnDemandUsdPerHour: 7.999, GpuCount: 8, GpuType: "L4", VcpuCount: 96, MemGb: 384},

	// GPU: A2 (A100 40GB)
	"a2-highgpu-1g":  {OnDemandUsdPerHour: 3.673, GpuCount: 1, GpuType: "A100", VcpuCount: 12, MemGb: 85},
	"a2-highgpu-2g":  {OnDemandUsdPerHour: 7.347, GpuCount: 2, GpuType: "A100", VcpuCount: 24, MemGb: 170},
	"a2-highgpu-4g":  {OnDemandUsdPerHour: 14.694, GpuCount: 4, GpuType: "A100", VcpuCount: 48, MemGb: 340},
	"a2-highgpu-8g":  {OnDemandUsdPerHour: 29.387, GpuCount: 8, GpuType: "A100", VcpuCount: 96, MemGb: 680},
	"a2-megagpu-16g": {OnDemandUsdPerHour: 55.74, GpuCount: 16, GpuType: "A100", VcpuCount: 96, MemGb: 1360},

	// GPU: A3 (H100 80GB)
	"a3-highgpu-8g": {OnDemandUsdPerHour: 88.494, GpuCount: 8, GpuType: "H100", VcpuCount: 208, MemGb: 1872},

	// GPU: A4 (B200)
	"a4-highgpu-8g": {OnDemandUsdPerHour: 90.22, GpuCount: 8, GpuType: "B200", VcpuCount: 224, MemGb: 3968},

	// GPU: G4 (RTX PRO 6000), whole-GPU sizes only
	"g4-standard-48":  {OnDemandUsdPerHour: 4.500, GpuCount: 1, GpuType: "RTX PRO 6000", VcpuCount: 48, MemGb: 180},
	"g4-standard-96":  {OnDemandUsdPerHour: 9.000, GpuCount: 2, GpuType: "RTX PRO 6000", VcpuCount: 96, MemGb: 360},
	"g4-standard-192": {OnDemandUsdPerHour: 18.000, GpuCount: 4, GpuType: "RTX PRO 6000", VcpuCount: 192, MemGb: 720},
	"g4-standard-384": {OnDemandUsdPerHour: 36.000, GpuCount: 8, GpuType: "RTX PRO 6000", VcpuCount: 384, MemGb: 1440},
}
