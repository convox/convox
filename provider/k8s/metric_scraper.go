package k8s

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type MetricScraperClient struct {
	c       *http.Client
	cluster kubernetes.Interface
	host    string
}

func NewMetricScraperClient(cluster kubernetes.Interface, host string) *MetricScraperClient {
	return &MetricScraperClient{
		c: &http.Client{
			Timeout: 10 * time.Second,
		},
		cluster: cluster,
		host:    host,
	}
}

func (m *MetricScraperClient) GetRackMetrics(opts structs.MetricsOptions) (structs.Metrics, error) {
	if m.host == "" {
		return nil, errors.WithStack(fmt.Errorf("unimplemented"))
	}

	ns, err := m.cluster.CoreV1().Nodes().List(am.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	nodeNames := []string{}
	var cpuAllocatable, memAllocatable float64
	for _, n := range ns.Items {
		nodeNames = append(nodeNames, n.ObjectMeta.Name)
		cpuAllocatable += float64(n.Status.Allocatable.Cpu().MilliValue())
		memAllocatable += float64(n.Status.Allocatable.Memory().Value())
	}

	cpus, err := m.GetNodesMetrics(strings.Join(nodeNames, ","), structs.ScraperMetricTypeCpu)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(cpus.Items) == 0 {
		return nil, fmt.Errorf("metrics unavailable")
	}

	for i := 1; i < len(cpus.Items); i++ {
		for j, d := range cpus.Items[i].MetricPoints {
			if j < len(cpus.Items[0].MetricPoints) {
				cpus.Items[0].MetricPoints[j].Value += d.Value
			}
		}
	}

	cpum := structs.Metric{
		Name: "cluster:cpu:utilization",
	}
	for _, d := range cpus.Items[0].MetricPoints {
		p := caculatePercentage(float64(d.Value), cpuAllocatable)
		cpum.Values = append(cpum.Values, structs.MetricValue{
			Average: p,
			Count:   1,
			Maximum: p,
			Minimum: p,
			Sum:     p,
			Time:    d.Timestamp,
		})
	}

	mems, err := m.GetNodesMetrics(strings.Join(nodeNames, ","), structs.ScraperMetricTypeMem)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for i := 1; i < len(mems.Items); i++ {
		for j, d := range mems.Items[i].MetricPoints {
			if j < len(mems.Items[0].MetricPoints) {
				mems.Items[0].MetricPoints[j].Value += d.Value
			}
		}
	}

	memm := structs.Metric{
		Name: "cluster:mem:utilization",
	}
	for _, d := range mems.Items[0].MetricPoints {
		p := caculatePercentage(float64(d.Value), memAllocatable)
		memm.Values = append(memm.Values, structs.MetricValue{
			Average: p,
			Count:   1,
			Maximum: p,
			Minimum: p,
			Sum:     p,
			Time:    d.Timestamp,
		})
	}

	if opts.Period != nil {
		cpum = aggregateMetricByPeriod(cpum, *opts.Period)
		memm = aggregateMetricByPeriod(memm, *opts.Period)
	}
	if opts.Start != nil {
		cpum = discradMetricByStart(cpum, *opts.Start)
		memm = discradMetricByStart(memm, *opts.Start)
	}

	return structs.Metrics{cpum, memm}, nil
}

// nodeNames: single or comma seperated node names
func (m *MetricScraperClient) GetNodesMetrics(nodeNames string, metricType structs.ScraperMetricType) (*structs.ScraperMetricList, error) {
	if m.host == "" {
		return nil, errors.WithStack(fmt.Errorf("unimplemented"))
	}

	resp, err := m.c.Get(fmt.Sprintf("%s/api/v1/dashboard/nodes/%s/metrics/%s/data", m.host, nodeNames, metricType))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.WithStack(fmt.Errorf("failed to get node metrics"))
	}

	data := &structs.ScraperMetricList{}
	if err := json.NewDecoder(resp.Body).Decode(data); err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}
