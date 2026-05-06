package structs

import _ "embed"

// KarpenterScrapeYAML is the canonical Karpenter scrape config used by both
// the paid kube-prometheus-stack chart and the free
// prometheus-community/prometheus chart. The kubernetes_sd_configs Pod-role
// approach works in BOTH charts uniformly (paid chart's ServiceMonitor CRD
// path is NOT required), so GPU observability dashboards relying on
// karpenter_* metrics work whether the rack runs paid metrics or the free
// lightweight chart.
//
// Karpenter pods live in kube-system per terraform/cluster/aws/karpenter.tf.
// Container port 8080 is the metrics endpoint; container port 8081 is the
// health-check endpoint and is excluded by the relabel on container
// port number.
//
//go:embed karpenter-scrape.yaml
var KarpenterScrapeYAML []byte
