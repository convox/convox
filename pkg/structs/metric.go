package structs

import "time"

type Metric struct {
	Name   string       `json:"name"`
	Values MetricValues `json:"values"`
}

type Metrics []Metric

type MetricValue struct {
	Average float64   `json:"avg"`
	Count   float64   `json:"count"`
	Maximum float64   `json:"max"`
	Minimum float64   `json:"min"`
	Sum     float64   `json:"sum"`
	Time    time.Time `json:"time"`
}

type MetricValues []MetricValue

type MetricsOptions struct {
	End     *time.Time `query:"end"`
	Metrics []string   `query:"metrics"`
	Start   *time.Time `query:"start"`
	Period  *int64     `query:"period"`
}

type ScraperMetricType string

const (
	ScraperMetricTypeCpu ScraperMetricType = "cpu"
	ScraperMetricTypeMem ScraperMetricType = "mem"
)

type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     uint64    `json:"value"`
}

type ScraperMetricList struct {
	Items []ScraperMetric `json:"items"`
}

type ScraperMetric struct {
	MetricPoints []MetricPoint `json:"metricPoints"`
	MetricName   string        `json:"metricName"`
}
