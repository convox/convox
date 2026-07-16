package main

import (
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/convox/convox/pkg/billing"
)

const (
	genDate      = "2026-07-16"
	docCheckDate = "2026-07-16"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) != 3 {
		log.Fatalf("usage: gen <azure|aws> <dataset.json>")
	}

	switch os.Args[1] {
	case "azure":
		genAzure(os.Args[2])
	case "aws":
		genAWS(os.Args[2])
	default:
		log.Fatalf("unknown subcommand %q (want azure or aws)", os.Args[1]) //nolint:gosec // CLI argument echoed back to the operator
	}
}

type azureRow struct {
	ArmSkuName  string  `json:"armSkuName"`
	MeterName   string  `json:"meterName"`
	ProductName string  `json:"productName"`
	RetailPrice float64 `json:"retailPrice"`
}

type cpuMeta struct {
	vcpu int
	mem  float64
}

type gpuMeta struct {
	vcpu  int
	mem   float64
	count int
	model string
}

var azureAliasDrops = map[string]bool{
	"Standard_ND96asr_A100_v4":     true,
	"Standard_ND96is_H100_v5":      true,
	"Standard_ND96is_flex_H100_v5": true,
	"Standard_ND96is_noIB_H100_v5": true,
	"Standard_ND96isrf_H100_v5":    true,
	// absent from both the VM size docs and the pricing calculator (checked 2026-07-14)
	"Standard_E96ias_v5":  true,
	"Standard_E96iads_v5": true,
	"Standard_F1as_v6":    true,
	"Standard_F72as_v6":   true,
}

var azureGpu = map[string]gpuMeta{
	"Standard_NC4as_T4_v3":      {4, 28, 1, "T4"},
	"Standard_NC8as_T4_v3":      {8, 56, 1, "T4"},
	"Standard_NC16as_T4_v3":     {16, 110, 1, "T4"},
	"Standard_NC64as_T4_v3":     {64, 440, 4, "T4"},
	"Standard_NC24ads_A100_v4":  {24, 220, 1, "A100"},
	"Standard_NC48ads_A100_v4":  {48, 440, 2, "A100"},
	"Standard_NC96ads_A100_v4":  {96, 880, 4, "A100"},
	"Standard_ND96asr_v4":       {96, 900, 8, "A100"},
	"Standard_ND96ams_A100_v4":  {96, 1900, 8, "A100"},
	"Standard_ND96amsr_A100_v4": {96, 1900, 8, "A100"},
	"Standard_NC40ads_H100_v5":  {40, 320, 1, "H100"},
	"Standard_NC80adis_H100_v5": {80, 640, 2, "H100"},
	"Standard_ND96isr_H100_v5":  {96, 1900, 8, "H100"},
	"Standard_NV6ads_A10_v5":    {6, 55, 1, "A10"},
	"Standard_NV12ads_A10_v5":   {12, 110, 1, "A10"},
	"Standard_NV18ads_A10_v5":   {18, 220, 1, "A10"},
	"Standard_NV36ads_A10_v5":   {36, 440, 1, "A10"},
	"Standard_NV36adms_A10_v5":  {36, 880, 1, "A10"},
	"Standard_NV72ads_A10_v5":   {72, 880, 2, "A10"},
}

var azureBv1 = map[string]cpuMeta{
	"Standard_B1ls":  {1, 0.5},
	"Standard_B1s":   {1, 1},
	"Standard_B1ms":  {1, 2},
	"Standard_B2s":   {2, 4},
	"Standard_B2ms":  {2, 8},
	"Standard_B4ms":  {4, 16},
	"Standard_B8ms":  {8, 32},
	"Standard_B12ms": {12, 48},
	"Standard_B16ms": {16, 64},
	"Standard_B20ms": {20, 80},
}

var azureBv2t = map[string]cpuMeta{
	"Standard_B2ts_v2":  {2, 1},
	"Standard_B2ats_v2": {2, 1},
	"Standard_B2pts_v2": {2, 1},
}

var azureDv2 = map[string]cpuMeta{
	"Standard_D1_v2":   {1, 3.5},
	"Standard_D2_v2":   {2, 7},
	"Standard_D3_v2":   {4, 14},
	"Standard_D4_v2":   {8, 28},
	"Standard_D5_v2":   {16, 56},
	"Standard_D11_v2":  {2, 14},
	"Standard_D12_v2":  {4, 28},
	"Standard_D13_v2":  {8, 56},
	"Standard_D14_v2":  {16, 112},
	"Standard_D15_v2":  {20, 140},
	"Standard_DS1_v2":  {1, 3.5},
	"Standard_DS2_v2":  {2, 7},
	"Standard_DS3_v2":  {4, 14},
	"Standard_DS4_v2":  {8, 28},
	"Standard_DS5_v2":  {16, 56},
	"Standard_DS11_v2": {2, 14},
	"Standard_DS12_v2": {4, 28},
	"Standard_DS13_v2": {8, 56},
	"Standard_DS14_v2": {16, 112},
	"Standard_DS15_v2": {20, 140},
}

var azureM = map[string]cpuMeta{
	"Standard_M8ms":   {8, 218.75},
	"Standard_M16ms":  {16, 437.5},
	"Standard_M32ts":  {32, 192},
	"Standard_M32ls":  {32, 256},
	"Standard_M32ms":  {32, 875},
	"Standard_M64ls":  {64, 512},
	"Standard_M64s":   {64, 1024},
	"Standard_M64ms":  {64, 1792},
	"Standard_M128s":  {128, 2048},
	"Standard_M128ms": {128, 3892},
}

type azureFamily struct {
	name    string
	comment string
	sel     *regexp.Regexp
}

var azureFamilies = []azureFamily{
	{"Bv1", "// General purpose: B v1", regexp.MustCompile(`^Standard_B\d+l?m?s$`)},
	{"Bv2", "// General purpose: B v2", regexp.MustCompile(`^Standard_B\d+(s|as|ps|als|ls|pls|ts|ats|pts)_v2$`)},
	{"Dv2", "// General purpose: Dv2 and DSv2", nil},
	{"Dv3", "// General purpose: D and Ds v3", regexp.MustCompile(`^Standard_D\d+s?_v3$`)},
	{"Dv4", "// General purpose: D, Dd, Ds, Dds v4", regexp.MustCompile(`^Standard_D\d+d?s?_v4$`)},
	{"Dv5", "// General purpose: D, Dd, Ds, Dds v5", regexp.MustCompile(`^Standard_D\d+d?s?_v5$`)},
	{"Dsv6", "// General purpose: Ds v6", regexp.MustCompile(`^Standard_D\d+s_v6$`)},
	{"Das", "// General purpose: Das v4 to v6 (AMD)", regexp.MustCompile(`^Standard_D\d+as_v[456]$`)},
	{"Dads", "// General purpose: Dads v5 to v6 (AMD)", regexp.MustCompile(`^Standard_D\d+ads_v[56]$`)},
	{"Dps", "// General purpose: Dps and Dpds v5 to v6 (ARM)", regexp.MustCompile(`^Standard_D\d+pd?s_v[56]$`)},
	{"Dls", "// General purpose: Dls and Dlds v5 to v6 (low-memory)", regexp.MustCompile(`^Standard_D\d+ld?s_v[56]$`)},
	{"Fsv2", "// Compute optimized: Fs v2", regexp.MustCompile(`^Standard_F\d+s_v2$`)},
	{"Fasv6", "// Compute optimized: Fas v6", regexp.MustCompile(`^Standard_F\d+as_v6$`)},
	{"Ev3", "// Memory optimized: E and Es v3", regexp.MustCompile(`^Standard_E\d+i?s?_v3$`)},
	{"Ev4", "// Memory optimized: E, Ed, Es, Eds v4", regexp.MustCompile(`^Standard_E\d+i?d?s?_v4$`)},
	{"Easv4", "// Memory optimized: Eas v4 (AMD)", regexp.MustCompile(`^Standard_E\d+as_v4$`)},
	{"Ev5", "// Memory optimized: E, Ed, Es, Eds v5", regexp.MustCompile(`^Standard_E\d+i?d?s?_v5$`)},
	{"Easv56", "// Memory optimized: Eas and Eads v5 to v6 (AMD)", regexp.MustCompile(`^Standard_E\d+i?ad?s_v[56]$`)},
	{"Esv6", "// Memory optimized: Es v6", regexp.MustCompile(`^Standard_E\d+i?s_v6$`)},
	{"Eps", "// Memory optimized: Eps and Epds v5 to v6 (ARM)", regexp.MustCompile(`^Standard_E\d+pd?s_v[56]$`)},
	{"M", "// Memory optimized: M series", nil},
	{"Lsv2", "// Storage optimized: Ls v2", regexp.MustCompile(`^Standard_L\d+s_v2$`)},
	{"Lsv3", "// Storage optimized: Ls v3", regexp.MustCompile(`^Standard_L\d+s_v3$`)},
	{"Lasv3", "// Storage optimized: Las v3 (AMD)", regexp.MustCompile(`^Standard_L\d+as_v3$`)},
	{"NCasT4", "// GPU: NCas T4 v3 (T4)", regexp.MustCompile(`^Standard_NC\d+as_T4_v3$`)},
	{"NCA100", "// GPU: NC A100 v4 (A100)", regexp.MustCompile(`^Standard_NC\d+ads_A100_v4$`)},
	{"NDA100", "// GPU: ND A100 v4 (A100)", regexp.MustCompile(`^Standard_ND96(asr_v4|ams_A100_v4|amsr_A100_v4)$`)},
	{"NCH100", "// GPU: NC H100 v5 (H100)", regexp.MustCompile(`^Standard_NC(40ads|80adis)_H100_v5$`)},
	{"NDH100", "// GPU: ND H100 v5 (H100)", regexp.MustCompile(`^Standard_ND96isr_H100_v5$`)},
	{"NVA10", "// GPU: NVads A10 v5 (A10)", regexp.MustCompile(`^Standard_NV\d+adm?s_A10_v5$`)},
}

var azureRatios = []struct {
	re    *regexp.Regexp
	ratio float64
}{
	{regexp.MustCompile(`^Standard_D\d+s?_v[345]$`), 4},
	{regexp.MustCompile(`^Standard_D\d+s_v6$`), 4},
	{regexp.MustCompile(`^Standard_D\d+ds?_v[45]$`), 4},
	{regexp.MustCompile(`^Standard_D\d+as_v[456]$`), 4},
	{regexp.MustCompile(`^Standard_D\d+ads_v[56]$`), 4},
	{regexp.MustCompile(`^Standard_D\d+pd?s_v[56]$`), 4},
	{regexp.MustCompile(`^Standard_F\d+as_v6$`), 4},
	{regexp.MustCompile(`^Standard_B\d+(a|p)?s_v2$`), 4},
	{regexp.MustCompile(`^Standard_F\d+s_v2$`), 2},
	{regexp.MustCompile(`^Standard_B\d+(a|p)?ls_v2$`), 2},
	{regexp.MustCompile(`^Standard_E\d+i?d?s?_v[345]$`), 8},
	{regexp.MustCompile(`^Standard_E\d+i?ad?s_v[456]$`), 8},
	{regexp.MustCompile(`^Standard_E\d+i?s_v6$`), 8},
	{regexp.MustCompile(`^Standard_E\d+pd?s_v[56]$`), 8},
	{regexp.MustCompile(`^Standard_D\d+ld?s_v[56]$`), 2},
	{regexp.MustCompile(`^Standard_L\d+a?s_v[23]$`), 8},
}

var azureMemDeviants = []struct {
	re  *regexp.Regexp
	mem float64
}{
	{regexp.MustCompile(`^Standard_E64\w*_v3$`), 432},
	{regexp.MustCompile(`^Standard_E(64|64d|64s|64ds|80is|80ids)_v4$`), 504},
	{regexp.MustCompile(`^Standard_E96as_v4$`), 672},
	{regexp.MustCompile(`^Standard_E(96|104i|112i)\w*_v5$`), 672},
	{regexp.MustCompile(`^Standard_E96ad?s_v6$`), 672},
	{regexp.MustCompile(`^Standard_E192i\w*_v6$`), 1832},
	{regexp.MustCompile(`^Standard_D64pd?s_v5$`), 208},
	{regexp.MustCompile(`^Standard_E32pd?s_v5$`), 208},
	{regexp.MustCompile(`^Standard_E96pd?s_v6$`), 672},
}

var (
	azureConstrained = regexp.MustCompile(`-\d`)
	azureCloud       = regexp.MustCompile(`Cloud ?Services`)
	azureVcpuDigits  = regexp.MustCompile(`^Standard_[A-Z]+(\d+)`)
)

func genAzure(path string) {
	rows := loadAzure(path)
	onDemand, fams, spotRows := selectAzureOnDemand(rows)

	type entry struct {
		price   float64
		spot    float64
		vcpu    int
		mem     float64
		gpu     int
		gpuType string
	}

	entries := map[string]entry{}
	for sku, row := range onDemand {
		vcpu, mem, gpu, gpuType := azureMeta(sku)
		entries[sku] = entry{price: row.RetailPrice, vcpu: vcpu, mem: mem, gpu: gpu, gpuType: gpuType}
	}

	for sku, factor := range azureSpotFactors(onDemand, spotRows) {
		e := entries[sku]
		e.spot = factor
		entries[sku] = e
	}

	byFamily := map[string][]string{}
	for sku := range entries {
		byFamily[fams[sku]] = append(byFamily[fams[sku]], sku)
	}
	for _, f := range azureFamilies {
		if len(byFamily[f.name]) == 0 {
			log.Fatalf("family coverage guard: no entries selected for family %s", f.name)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by go run ./pkg/billing/gen azure on %s from %s. DO NOT EDIT by hand.\n", genDate, filepath.Base(path))
	fmt.Fprintf(&b, "// Metadata cross-checked against learn.microsoft.com VM size documentation on %s.\n\n", docCheckDate)
	b.WriteString("package billing\n\nvar AzureInstancePricing = map[string]InstancePrice{\n")

	spotCount := 0
	for _, f := range azureFamilies {
		skus := byFamily[f.name]
		sort.Slice(skus, func(i, j int) bool {
			a, z := entries[skus[i]], entries[skus[j]]
			if a.vcpu != z.vcpu {
				return a.vcpu < z.vcpu
			}
			if a.mem != z.mem {
				return a.mem < z.mem
			}
			return skus[i] < skus[j]
		})
		b.WriteString("\t" + f.comment + "\n")
		for _, sku := range skus {
			e := entries[sku]
			fmt.Fprintf(&b, "\t%q: {OnDemandUsdPerHour: %s", sku, ffmt(e.price))
			if e.spot > 0 {
				fmt.Fprintf(&b, ", SpotUsdPerHourFactor: %s", ffmt(e.spot))
				spotCount++
			}
			if e.gpu > 0 {
				fmt.Fprintf(&b, ", GpuCount: %d, GpuType: %q", e.gpu, e.gpuType)
			}
			fmt.Fprintf(&b, ", VcpuCount: %d, MemGb: %s},\n", e.vcpu, ffmt(e.mem))
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		log.Fatalf("formatting generated azure source: %v", err)
	}
	if _, err := os.Stdout.Write(src); err != nil {
		log.Fatalf("writing azure output: %v", err)
	}

	fmt.Fprintf(os.Stderr, "azure: %d dataset rows, %d entries emitted\n", len(rows), len(entries))
	for _, f := range azureFamilies {
		fmt.Fprintf(os.Stderr, "  %-8s %d\n", f.name, len(byFamily[f.name]))
	}
	gpuCount := 0
	for sku := range entries {
		if _, ok := azureGpu[sku]; ok {
			gpuCount++
		}
	}
	fmt.Fprintf(os.Stderr, "azure: %d GPU SKUs of %d curated, %d with spot factors\n", gpuCount, len(azureGpu), spotCount)
}

func loadAzure(path string) []azureRow {
	data, err := os.ReadFile(path) //nolint:gosec // dataset path is an operator-supplied CLI argument
	if err != nil {
		log.Fatalf("reading azure dataset: %v", err)
	}

	var rows []azureRow
	if err := json.Unmarshal(data, &rows); err != nil {
		log.Fatalf("decoding azure dataset: %v", err)
	}

	return rows
}

func azureFamilyFor(sku string) (string, bool) {
	if _, ok := azureM[sku]; ok {
		return "M", true
	}
	if _, ok := azureDv2[sku]; ok {
		return "Dv2", true
	}
	for _, f := range azureFamilies {
		if f.sel != nil && f.sel.MatchString(sku) {
			return f.name, true
		}
	}
	return "", false
}

func selectAzureOnDemand(rows []azureRow) (map[string]azureRow, map[string]string, map[string][]azureRow) {
	onDemand := map[string]azureRow{}
	fams := map[string]string{}
	spotRows := map[string][]azureRow{}

	for _, r := range rows {
		if !strings.HasPrefix(r.ArmSkuName, "Standard_") {
			continue
		}
		if strings.Contains(r.MeterName, "Low Priority") {
			continue
		}
		if strings.Contains(r.ProductName, "Windows") || strings.Contains(r.ProductName, "Dedicated Host") || azureCloud.MatchString(r.ProductName) {
			continue
		}
		if azureConstrained.MatchString(r.ArmSkuName) {
			continue
		}
		if azureAliasDrops[r.ArmSkuName] {
			continue
		}
		if strings.Contains(r.MeterName, "Spot") {
			spotRows[r.ArmSkuName] = append(spotRows[r.ArmSkuName], r)
			continue
		}
		if r.RetailPrice == 0 {
			continue
		}
		fam, ok := azureFamilyFor(r.ArmSkuName)
		if !ok {
			continue
		}
		if prev, ok := onDemand[r.ArmSkuName]; ok {
			log.Fatalf("duplicate on-demand rows for %s: (%s / %s / %v) and (%s / %s / %v)",
				r.ArmSkuName, prev.ProductName, prev.MeterName, prev.RetailPrice, r.ProductName, r.MeterName, r.RetailPrice)
		}
		if r.RetailPrice < 0 {
			log.Fatalf("negative price for %s: %v", r.ArmSkuName, r.RetailPrice)
		}
		onDemand[r.ArmSkuName] = r
		fams[r.ArmSkuName] = fam
	}

	return onDemand, fams, spotRows
}

func azureMeta(sku string) (int, float64, int, string) {
	if g, ok := azureGpu[sku]; ok {
		if g.count < 1 || g.model == "" {
			log.Fatalf("curated GPU SKU %s has invalid metadata: count %d model %q", sku, g.count, g.model)
		}
		return g.vcpu, g.mem, g.count, g.model
	}
	if strings.HasPrefix(sku, "Standard_NC") || strings.HasPrefix(sku, "Standard_ND") || strings.HasPrefix(sku, "Standard_NV") {
		log.Fatalf("GPU SKU %s selected but missing curated metadata", sku)
	}
	for _, cur := range []map[string]cpuMeta{azureBv1, azureBv2t, azureDv2, azureM} {
		if m, ok := cur[sku]; ok {
			return m.vcpu, m.mem, 0, ""
		}
	}

	var ratio float64
	for _, r := range azureRatios {
		if r.re.MatchString(sku) {
			ratio = r.ratio
			break
		}
	}
	if ratio == 0 {
		log.Fatalf("selected SKU %s matches no ratio rule and no curated entry", sku)
	}

	m := azureVcpuDigits.FindStringSubmatch(sku)
	if m == nil {
		log.Fatalf("cannot derive vcpu count from SKU name %s", sku)
	}
	vcpu, err := strconv.Atoi(m[1])
	if err != nil || vcpu <= 0 {
		log.Fatalf("invalid vcpu digits in SKU name %s", sku)
	}

	mem := float64(vcpu) * ratio
	for _, d := range azureMemDeviants {
		if d.re.MatchString(sku) {
			mem = d.mem
			break
		}
	}

	return vcpu, mem, 0, ""
}

func azureSpotFactors(onDemand map[string]azureRow, spotRows map[string][]azureRow) map[string]float64 {
	factors := map[string]float64{}

	for sku := range azureGpu {
		od, ok := onDemand[sku]
		if !ok {
			log.Fatalf("curated GPU SKU %s missing from selected on-demand rows", sku)
		}
		var priced []azureRow
		for _, r := range spotRows[sku] {
			if r.RetailPrice > 0 {
				priced = append(priced, r)
			}
		}
		if len(priced) > 1 {
			var details []string
			for _, r := range priced {
				details = append(details, fmt.Sprintf("%s / %s / %v", r.ProductName, r.MeterName, r.RetailPrice))
			}
			log.Fatalf("multiple spot rows for GPU SKU %s: %s", sku, strings.Join(details, "; "))
		}
		if len(priced) == 0 {
			continue
		}
		factor := priced[0].RetailPrice / od.RetailPrice
		if factor <= 0 || factor >= 1 {
			log.Fatalf("spot factor for %s out of range (0,1): %v", sku, factor)
		}
		rounded := math.Round(factor*10000) / 10000
		if rounded <= 0 || rounded >= 1 {
			log.Fatalf("rounded spot factor for %s out of range (0,1): %v", sku, rounded)
		}
		factors[sku] = rounded
	}

	return factors
}

type awsAttrs struct {
	InstanceType    string `json:"instanceType"`
	OperatingSystem string `json:"operatingSystem"`
	Tenancy         string `json:"tenancy"`
	PreInstalledSw  string `json:"preInstalledSw"`
	CapacityStatus  string `json:"capacitystatus"`
	MarketOption    string `json:"marketoption"`
	Vcpu            string `json:"vcpu"`
	Memory          string `json:"memory"`
	Gpu             string `json:"gpu"`
}

type awsProduct struct {
	Attributes awsAttrs `json:"attributes"`
}

type awsKept struct {
	sku   string
	attrs awsAttrs
}

var awsGpuModels = map[string]string{
	"g4dn": "T4",
	"g4ad": "V520",
	"g5":   "A10G",
	"g5g":  "T4G",
	"g6":   "L4",
	"g6e":  "L40S",
	"p3":   "V100",
	"p4d":  "A100",
	"p5":   "H100",
	"inf1": "Inferentia1",
	"inf2": "Inferentia2",
	"trn1": "Trainium1",
}

var awsNewFamilies = map[string]bool{
	"m7a": true, "c7a": true, "r7a": true, "g6e": true, "t4g": true,
	"m6g": true, "c6g": true, "r6g": true,
	"m7g": true, "c7g": true, "r7g": true,
	"m8g": true, "c8g": true, "r8g": true,
	"c5a": true, "c6a": true, "c5n": true, "c6in": true,
	"m5n": true, "m6in": true,
	"r5a": true, "r6a": true, "r5n": true, "r6in": true,
	"g4ad": true, "g5g": true,
	"i3": true, "i3en": true, "i4i": true, "i4g": true,
	"im4gn": true, "is4gen": true, "d3": true, "d3en": true,
	"x2idn": true, "x2iedn": true, "x2gd": true, "z1d": true,
}

var awsFamilyComments = []struct {
	family  string
	comment string
}{
	{"g4dn", "// GPU: g4dn (T4)"},
	{"g4ad", "// GPU: g4ad (V520)"},
	{"g5", "// GPU: g5 (A10G)"},
	{"g5g", "// GPU: g5g (T4G)"},
	{"g6", "// GPU: g6 (L4)"},
	{"g6e", "// GPU: g6e (L40S)"},
	{"p3", "// GPU: p3 (V100)"},
	{"p4d", "// GPU: p4d (A100)"},
	{"p5", "// GPU: p5 (H100)"},
	{"inf1", "// Neuron: inf1 (Inferentia1)"},
	{"inf2", "// Neuron: inf2 (Inferentia2)"},
	{"trn1", "// Neuron: trn1 (Trainium1)"},
	{"m5", "// CPU general: m5"},
	{"m5a", "// CPU general: m5a"},
	{"m5n", "// CPU general: m5n (network-optimized)"},
	{"m6i", "// CPU general: m6i"},
	{"m6a", "// CPU general: m6a"},
	{"m6in", "// CPU general: m6in (network-optimized)"},
	{"m7i", "// CPU general: m7i"},
	{"m7a", "// CPU general: m7a"},
	{"m6g", "// CPU general: m6g (Graviton2)"},
	{"m7g", "// CPU general: m7g (Graviton3)"},
	{"m8g", "// CPU general: m8g (Graviton4)"},
	{"c5", "// CPU compute: c5"},
	{"c5a", "// CPU compute: c5a (AMD-EPYC)"},
	{"c5n", "// CPU compute: c5n (network-optimized)"},
	{"c6i", "// CPU compute: c6i"},
	{"c6a", "// CPU compute: c6a (AMD-EPYC)"},
	{"c6in", "// CPU compute: c6in (network-optimized)"},
	{"c7i", "// CPU compute: c7i"},
	{"c7a", "// CPU compute: c7a"},
	{"c6g", "// CPU compute: c6g (Graviton2)"},
	{"c7g", "// CPU compute: c7g (Graviton3)"},
	{"c8g", "// CPU compute: c8g (Graviton4)"},
	{"r5", "// CPU memory: r5"},
	{"r5a", "// CPU memory: r5a (AMD-EPYC)"},
	{"r5n", "// CPU memory: r5n (network-optimized)"},
	{"r6i", "// CPU memory: r6i"},
	{"r6a", "// CPU memory: r6a (AMD-EPYC)"},
	{"r6in", "// CPU memory: r6in (network-optimized)"},
	{"r7i", "// CPU memory: r7i"},
	{"r7a", "// CPU memory: r7a"},
	{"r6g", "// CPU memory: r6g (Graviton2)"},
	{"r7g", "// CPU memory: r7g (Graviton3)"},
	{"r8g", "// CPU memory: r8g (Graviton4)"},
	{"x2idn", "// CPU memory: x2idn (Intel high-memory)"},
	{"x2iedn", "// CPU memory: x2iedn (Intel high-memory, extended)"},
	{"x2gd", "// CPU memory: x2gd (Graviton2 high-memory)"},
	{"z1d", "// CPU memory: z1d (high-frequency)"},
	{"t2", "// CPU general: t2 (legacy burstable, no Nitro)"},
	{"t3", "// CPU general: t3 (Nitro burstable, AMD64)"},
	{"t3a", "// CPU general: t3a (AMD-EPYC variant of t3)"},
	{"t4g", "// CPU general: t4g (Graviton2 burstable)"},
	{"i3", "// Storage optimized: i3 (NVMe SSD)"},
	{"i3en", "// Storage optimized: i3en (dense NVMe SSD)"},
	{"i4i", "// Storage optimized: i4i (Intel NVMe SSD)"},
	{"i4g", "// Storage optimized: i4g (Graviton2 NVMe SSD)"},
	{"im4gn", "// Storage optimized: im4gn (Graviton2 NVMe SSD)"},
	{"is4gen", "// Storage optimized: is4gen (Graviton2 dense NVMe SSD)"},
	{"d3", "// Storage optimized: d3 (dense HDD)"},
	{"d3en", "// Storage optimized: d3en (dense HDD)"},
	{"m4", "// CPU general: m4 (legacy, pre-Nitro)"},
	{"c4", "// CPU compute: c4 (legacy)"},
	{"r4", "// CPU memory: r4 (legacy)"},
}

func genAWS(path string) {
	f, err := os.Open(path) //nolint:gosec // dataset path is an operator-supplied CLI argument
	if err != nil {
		log.Fatalf("opening aws offers file: %v", err)
	}

	dec := json.NewDecoder(f)
	expectDelim(dec, '{')

	var byType map[string]*awsKept
	var prices map[string]float64

	for dec.More() {
		key := stringToken(dec)
		switch key {
		case "products":
			byType = loadAWSProducts(dec)
		case "terms":
			if byType == nil {
				log.Fatalf("terms section appeared before products in offers file")
			}
			keep := map[string]bool{}
			for _, k := range byType {
				keep[k.sku] = true
			}
			prices = loadAWSPrices(dec, keep)
		default:
			skipValue(dec)
		}
	}
	if byType == nil || prices == nil {
		log.Fatalf("offers file missing products or terms section")
	}
	if err := f.Close(); err != nil {
		log.Fatalf("closing aws offers file: %v", err)
	}

	emitAWS(path, byType, prices)
}

func loadAWSProducts(dec *json.Decoder) map[string]*awsKept {
	expectDelim(dec, '{')

	byType := map[string]*awsKept{}
	total := 0

	for dec.More() {
		sku := stringToken(dec)
		var p awsProduct
		if err := dec.Decode(&p); err != nil {
			log.Fatalf("decoding product %s: %v", sku, err) //nolint:gosec // guard detail from the dataset, printed to the operator's terminal
		}
		total++

		a := p.Attributes
		if a.InstanceType == "" || a.OperatingSystem != "Linux" || a.Tenancy != "Shared" ||
			a.PreInstalledSw != "NA" || a.CapacityStatus != "Used" || a.MarketOption != "OnDemand" {
			continue
		}
		if prev, ok := byType[a.InstanceType]; ok {
			log.Fatalf("duplicate product SKUs for instanceType %s: %s and %s", a.InstanceType, prev.sku, sku) //nolint:gosec // guard detail from the dataset, printed to the operator's terminal
		}
		byType[a.InstanceType] = &awsKept{sku: sku, attrs: a}
	}
	expectDelim(dec, '}')

	fmt.Fprintf(os.Stderr, "aws: %d products scanned, %d instance types survive filter\n", total, len(byType))

	return byType
}

func loadAWSPrices(dec *json.Decoder, keep map[string]bool) map[string]float64 {
	type awsTerm struct {
		PriceDimensions map[string]struct {
			Unit         string            `json:"unit"`
			PricePerUnit map[string]string `json:"pricePerUnit"`
		} `json:"priceDimensions"`
	}

	expectDelim(dec, '{')

	prices := map[string]float64{}

	for dec.More() {
		termType := stringToken(dec)
		if termType != "OnDemand" {
			skipValue(dec)
			continue
		}
		expectDelim(dec, '{')
		for dec.More() {
			sku := stringToken(dec)
			if !keep[sku] {
				skipValue(dec)
				continue
			}
			var terms map[string]awsTerm
			if err := dec.Decode(&terms); err != nil {
				log.Fatalf("decoding OnDemand terms for %s: %v", sku, err) //nolint:gosec // guard detail from the dataset, printed to the operator's terminal
			}
			var found []float64
			for _, t := range terms {
				for _, d := range t.PriceDimensions {
					if d.Unit != "Hrs" {
						continue
					}
					usd, ok := d.PricePerUnit["USD"]
					if !ok {
						continue
					}
					v, err := strconv.ParseFloat(usd, 64)
					if err != nil {
						log.Fatalf("parsing USD price %q for %s: %v", usd, sku, err) //nolint:gosec // guard detail from the dataset, printed to the operator's terminal
					}
					found = append(found, v)
				}
			}
			if len(found) != 1 {
				log.Fatalf("expected exactly one hourly USD price for %s, found %d: %v", sku, len(found), found) //nolint:gosec // guard detail from the dataset, printed to the operator's terminal
			}
			prices[sku] = found[0]
		}
		expectDelim(dec, '}')
	}
	expectDelim(dec, '}')

	return prices
}

func emitAWS(path string, byType map[string]*awsKept, prices map[string]float64) {
	type entry struct {
		price   float64
		vcpu    int
		mem     float64
		gpu     int
		gpuType string
	}

	entries := map[string]entry{}
	for it, k := range byType {
		if strings.Contains(it, "-") {
			continue
		}
		fam := strings.SplitN(it, ".", 2)[0]
		if _, existing := billing.InstancePricing[it]; !existing && !awsNewFamilies[fam] {
			continue
		}

		price, ok := prices[k.sku]
		if !ok {
			log.Fatalf("no OnDemand price found for %s (product sku %s)", it, k.sku)
		}
		if price <= 0 {
			log.Fatalf("zero or negative price for %s: %v", it, price)
		}

		vcpu, err := strconv.Atoi(k.attrs.Vcpu)
		if err != nil || vcpu <= 0 {
			log.Fatalf("invalid vcpu attribute %q for %s", k.attrs.Vcpu, it)
		}
		mem := parseAWSMemory(k.attrs.Memory, it)

		gpu := 0
		gpuType := ""
		if k.attrs.Gpu != "" {
			gpu, err = strconv.Atoi(k.attrs.Gpu)
			if err != nil || gpu < 0 {
				log.Fatalf("invalid gpu attribute %q for %s", k.attrs.Gpu, it)
			}
		}
		if gpu > 0 {
			gpuType, ok = awsGpuModels[fam]
			if !ok {
				log.Fatalf("instanceType %s has gpu attribute %d but family %s is absent from the GPU model map", it, gpu, fam)
			}
		}

		entries[it] = entry{price: price, vcpu: vcpu, mem: mem, gpu: gpu, gpuType: gpuType}
	}

	var missing []string
	for it := range billing.InstancePricing {
		if _, ok := entries[it]; !ok {
			missing = append(missing, it)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		log.Fatalf("existing keys missing from refreshed table: %v", missing)
	}

	byFamily := map[string][]string{}
	for it := range entries {
		fam := strings.SplitN(it, ".", 2)[0]
		byFamily[fam] = append(byFamily[fam], it)
	}
	known := map[string]bool{}
	for _, fc := range awsFamilyComments {
		known[fc.family] = true
		if len(byFamily[fc.family]) == 0 {
			log.Fatalf("family coverage guard: no entries emitted for family %s", fc.family)
		}
	}
	for fam := range byFamily {
		if !known[fam] {
			log.Fatalf("emitted family %s has no section in the family comment list", fam)
		}
	}

	var b strings.Builder
	b.WriteString("package billing\n\n")
	fmt.Fprintf(&b, "// Code generated by go run ./pkg/billing/gen aws on %s from %s. DO NOT EDIT by hand.\n", genDate, filepath.Base(path))
	b.WriteString("var InstancePricing = map[string]InstancePrice{\n")

	for _, fc := range awsFamilyComments {
		its := byFamily[fc.family]
		sort.Slice(its, func(i, j int) bool {
			a, z := entries[its[i]], entries[its[j]]
			if a.vcpu != z.vcpu {
				return a.vcpu < z.vcpu
			}
			if a.mem != z.mem {
				return a.mem < z.mem
			}
			return its[i] < its[j]
		})
		b.WriteString("\t" + fc.comment + "\n")
		for _, it := range its {
			e := entries[it]
			fmt.Fprintf(&b, "\t%q: {OnDemandUsdPerHour: %s", it, ffmt(e.price))
			if e.gpu > 0 {
				fmt.Fprintf(&b, ", GpuCount: %d, GpuType: %q", e.gpu, e.gpuType)
			}
			fmt.Fprintf(&b, ", VcpuCount: %d, MemGb: %s},\n", e.vcpu, ffmt(e.mem))
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		log.Fatalf("formatting generated aws source: %v", err)
	}
	out := string(src)
	out = strings.TrimPrefix(out, "package billing\n\n")
	if _, err := os.Stdout.WriteString(out); err != nil {
		log.Fatalf("writing aws output: %v", err)
	}

	oldNew := map[string][2]float64{}
	for it, e := range entries {
		if old, ok := billing.InstancePricing[it]; ok {
			oldNew[it] = [2]float64{old.OnDemandUsdPerHour, e.price}
		}
	}
	metaChanges := []string{}
	for it, e := range entries {
		old, ok := billing.InstancePricing[it]
		if !ok {
			continue
		}
		if old.VcpuCount != e.vcpu || old.MemGb != e.mem || old.GpuCount != e.gpu || old.GpuType != e.gpuType {
			metaChanges = append(metaChanges, fmt.Sprintf("%s: vcpu %d -> %d, mem %s -> %s, gpu %d %q -> %d %q",
				it, old.VcpuCount, e.vcpu, ffmt(old.MemGb), ffmt(e.mem), old.GpuCount, old.GpuType, e.gpu, e.gpuType))
		}
	}
	delta(oldNew, len(entries), metaChanges)
}

func delta(oldNew map[string][2]float64, totalEntries int, metaChanges []string) {
	keys := make([]string, 0, len(oldNew))
	for k := range oldNew {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var changedPcts []float64
	maxPct := 0.0
	maxKey := ""
	changed := 0

	fmt.Fprintf(os.Stderr, "AWS price table delta report (%s refresh vs %s table)\n", genDate, billing.PricingTableVersion())
	fmt.Fprintf(os.Stderr, "existing keys refreshed: %d, total entries in new table: %d, added: %d\n\n", len(oldNew), totalEntries, totalEntries-len(oldNew))

	for _, k := range keys {
		old, nw := oldNew[k][0], oldNew[k][1]
		if old == nw {
			continue
		}
		changed++
		pct := (nw - old) / old * 100
		changedPcts = append(changedPcts, math.Abs(pct))
		if math.Abs(pct) > math.Abs(maxPct) {
			maxPct = pct
			maxKey = k
		}
		fmt.Fprintf(os.Stderr, "CHANGED %-14s %12s -> %-12s %+.2f%%\n", k, ffmt(old), ffmt(nw), pct)
	}

	fmt.Fprintf(os.Stderr, "\nchanged: %d of %d existing keys, unchanged: %d\n", changed, len(oldNew), len(oldNew)-changed)
	if changed > 0 {
		sort.Float64s(changedPcts)
		median := changedPcts[len(changedPcts)/2]
		if len(changedPcts)%2 == 0 {
			median = (changedPcts[len(changedPcts)/2-1] + changedPcts[len(changedPcts)/2]) / 2
		}
		fmt.Fprintf(os.Stderr, "max delta: %+.2f%% (%s), median abs delta of changed keys: %.2f%%\n", maxPct, maxKey, median)
	}
	if len(metaChanges) > 0 {
		sort.Strings(metaChanges)
		fmt.Fprintf(os.Stderr, "\nmetadata changes:\n")
		for _, m := range metaChanges {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
	}
}

func parseAWSMemory(s, it string) float64 {
	trimmed := strings.TrimSuffix(strings.TrimSpace(s), " GiB")
	trimmed = strings.ReplaceAll(trimmed, ",", "")
	mem, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || mem <= 0 {
		log.Fatalf("invalid memory attribute %q for %s", s, it)
	}
	return mem
}

func ffmt(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func expectDelim(dec *json.Decoder, d json.Delim) {
	tok, err := dec.Token()
	if err != nil {
		log.Fatalf("reading token: %v", err)
	}
	if got, ok := tok.(json.Delim); !ok || got != d {
		log.Fatalf("expected delimiter %v, got %v", d, tok) //nolint:gosec // guard detail from the dataset, printed to the operator's terminal
	}
}

func stringToken(dec *json.Decoder) string {
	tok, err := dec.Token()
	if err != nil {
		log.Fatalf("reading token: %v", err)
	}
	s, ok := tok.(string)
	if !ok {
		log.Fatalf("expected string token, got %v", tok) //nolint:gosec // guard detail from the dataset, printed to the operator's terminal
	}
	return s
}

func skipValue(dec *json.Decoder) {
	tok, err := dec.Token()
	if err != nil {
		log.Fatalf("reading token: %v", err)
	}
	if d, ok := tok.(json.Delim); ok && (d == '{' || d == '[') {
		for dec.More() {
			skipValue(dec)
		}
		if _, err := dec.Token(); err != nil {
			log.Fatalf("reading closing delimiter: %v", err)
		}
	}
}
