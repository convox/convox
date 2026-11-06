package structs

import _ "embed"

// Karpenter Prometheus scrape config, works with both paid and free charts.
//
//go:embed karpenter-scrape.yaml
var KarpenterScrapeYAML []byte
