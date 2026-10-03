package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	costHistoryConfigMap   = "convox-cost-history"
	costHistoryDataKey     = "history.json"
	costHistoryDayLayout   = "2006-01-02"
	costHistoryMaxBytes    = 900 * 1024
	costHistoryMaxServices = 50
	costHistoryOther       = "_other"
)

type costHistoryDay struct {
	SpendUsd float64            `json:"spend-usd"`
	Services map[string]float64 `json:"services,omitempty"`
}

func costHistoryRetentionDays() int {
	n, err := strconv.Atoi(os.Getenv("COST_TRACKING_HISTORY_DAYS"))
	if err != nil || n < 31 {
		return 62
	}
	if n > 400 {
		return 400
	}
	return n
}

func costHistoryCutoff(now time.Time) string {
	return now.UTC().AddDate(0, 0, 1-costHistoryRetentionDays()).Format(costHistoryDayLayout)
}

func roundUsd(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func parseCostHistory(cm *v1.ConfigMap) (map[string]costHistoryDay, error) {
	days := map[string]costHistoryDay{}
	raw := cm.Data[costHistoryDataKey]
	if raw == "" {
		return days, nil
	}
	if err := json.Unmarshal([]byte(raw), &days); err != nil {
		return map[string]costHistoryDay{}, err
	}
	if days == nil {
		days = map[string]costHistoryDay{}
	}
	return days, nil
}

func (p *Provider) recordCostHistory(ctx context.Context, app string, lastTick, now time.Time, delta float64, perSvc map[string]float64) {
	if delta <= 0 {
		return
	}

	now = now.UTC()
	start := now.Add(-budgetMaxPollInterval)
	if lastTick.After(start) {
		start = lastTick.UTC()
	}
	shares := map[string]float64{now.Format(costHistoryDayLayout): 1}
	if midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC); start.Before(midnight) {
		before := midnight.Sub(start).Seconds() / now.Sub(start).Seconds()
		shares[start.Format(costHistoryDayLayout)] = before
		shares[now.Format(costHistoryDayLayout)] = 1 - before
	}

	services := make([]string, 0, len(perSvc))
	for svc := range perSvc {
		services = append(services, svc)
	}
	sort.Slice(services, func(i, j int) bool {
		if perSvc[services[i]] != perSvc[services[j]] {
			return perSvc[services[i]] > perSvc[services[j]]
		}
		return services[i] < services[j]
	})

	add := func(days map[string]costHistoryDay) {
		for date, share := range shares {
			day := days[date]
			if day.Services == nil {
				day.Services = map[string]float64{}
			}
			named := 0
			for svc := range day.Services {
				if !costHistoryOwnBucket(svc) {
					named++
				}
			}
			day.SpendUsd = roundUsd(day.SpendUsd + delta*share)
			for _, svc := range services {
				key := svc
				if _, ok := day.Services[svc]; !ok && !costHistoryOwnBucket(svc) {
					if named >= costHistoryMaxServices {
						key = costHistoryOther
					} else {
						named++
					}
				}
				day.Services[key] = roundUsd(day.Services[key] + perSvc[svc]*share)
			}
			days[date] = day
		}
	}

	if err := p.writeCostHistory(ctx, app, now, add); err != nil {
		fmt.Printf("ns=budget_accumulator at=history_write app=%s error=%q\n", app, err)
	}
}

func costHistoryOwnBucket(svc string) bool {
	return svc == perServiceBucketBuild || svc == perServiceBucketUnattributed || svc == costHistoryOther
}

func (p *Provider) writeCostHistory(ctx context.Context, app string, now time.Time, add func(map[string]costHistoryDay)) error {
	ns := p.AppNamespace(app)
	cms := p.Cluster.CoreV1().ConfigMaps(ns)

	for i := 0; i < budgetWriteConflictRetries; i++ {
		cm, err := cms.Get(ctx, costHistoryConfigMap, am.GetOptions{})
		create := ae.IsNotFound(err)
		if err != nil && !create {
			return errors.WithStack(err)
		}
		if create {
			cm = &v1.ConfigMap{ObjectMeta: am.ObjectMeta{
				Name:      costHistoryConfigMap,
				Namespace: ns,
				Labels:    map[string]string{"system": "convox", "rack": p.Name, "app": app, "type": "cost-history"},
			}}
		}

		days, err := parseCostHistory(cm)
		if err != nil {
			fmt.Printf("ns=budget_accumulator at=history_parse app=%s error=%q\n", app, err)
		}
		add(days)

		trimmed, err := trimCostHistory(days, costHistoryCutoff(now))
		if err != nil {
			return err
		}
		if trimmed > 0 {
			fmt.Printf("ns=budget_accumulator at=history_trimmed app=%s days=%d\n", app, trimmed)
		}

		data, err := json.Marshal(days)
		if err != nil {
			return errors.WithStack(err)
		}
		cm.Data = map[string]string{costHistoryDataKey: string(data)}

		if create {
			_, err = cms.Create(ctx, cm, am.CreateOptions{})
		} else {
			_, err = cms.Update(ctx, cm, am.UpdateOptions{})
		}
		if ae.IsConflict(err) || ae.IsAlreadyExists(err) {
			continue
		}
		return errors.WithStack(err)
	}

	return fmt.Errorf("failed to write cost history after %d retries", budgetWriteConflictRetries)
}

// trimCostHistory returns the number of days dropped for size, not for retention.
func trimCostHistory(days map[string]costHistoryDay, cutoff string) (int, error) {
	dates := make([]string, 0, len(days))
	for date := range days {
		if date < cutoff {
			delete(days, date)
			continue
		}
		dates = append(dates, date)
	}
	sort.Strings(dates)

	size := 1
	sizes := make([]int, len(dates))
	for i, date := range dates {
		data, err := json.Marshal(days[date])
		if err != nil {
			return 0, errors.WithStack(err)
		}
		sizes[i] = len(date) + len(data) + 4
		size += sizes[i]
	}

	trimmed := 0
	for ; trimmed < len(dates)-1 && size > costHistoryMaxBytes; trimmed++ {
		size -= sizes[trimmed]
		delete(days, dates[trimmed])
	}

	return trimmed, nil
}

func (p *Provider) AppCostWithOptions(app string, opts structs.AppCostOptions) (*structs.AppCost, error) {
	if opts.Start == nil && opts.End == nil {
		return p.AppCost(app)
	}

	start, err := parseCostHistoryDate(opts.Start, "start")
	if err != nil {
		return nil, err
	}
	end, err := parseCostHistoryDate(opts.End, "end")
	if err != nil {
		return nil, err
	}
	if start != "" && end != "" && start > end {
		return nil, errors.WithStack(structs.ErrBadRequest("start must not be after end"))
	}

	cost, err := p.AppCost(app)
	if err != nil {
		return nil, err
	}

	days := map[string]costHistoryDay{}
	cm, err := p.Cluster.CoreV1().ConfigMaps(p.AppNamespace(app)).Get(context.TODO(), costHistoryConfigMap, am.GetOptions{})
	switch {
	case ae.IsNotFound(err):
	case err != nil:
		return nil, errors.WithStack(err)
	default:
		days, _ = parseCostHistory(cm)
	}

	now := time.Now().UTC()
	cutoff := costHistoryCutoff(now)
	dates := []string{}
	for date := range days {
		if date >= cutoff {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)

	history := ""
	if len(dates) > 0 {
		history = dates[0]
	}
	if end == "" {
		end = now.Format(costHistoryDayLayout)
		if start > end {
			end = start
		}
	}
	if start == "" {
		start = history
		if start == "" || start > end {
			start = end
		}
	}

	var spend float64
	perSvc := map[string]float64{}
	for _, date := range dates {
		if date < start || date > end {
			continue
		}
		spend += days[date].SpendUsd
		for svc, v := range days[date].Services {
			perSvc[svc] += v
		}
	}

	instanceTypes := map[string]string{}
	for _, line := range cost.Breakdown {
		instanceTypes[line.Service] = line.InstanceType
	}
	for svc, v := range perSvc {
		perSvc[svc] = roundUsd(v)
	}

	cost.SpendUsd = roundUsd(spend)
	cost.Breakdown = buildBreakdown(&structs.AppBudgetState{PerServiceSpendUsd: perSvc, PerServiceInstanceType: instanceTypes})
	cost.VariantBreakdown = nil
	cost.RangeStart = start
	cost.RangeEnd = end
	cost.HistoryStart = history

	return cost, nil
}

func parseCostHistoryDate(v *string, name string) (string, error) {
	if v == nil {
		return "", nil
	}
	if _, err := time.Parse(costHistoryDayLayout, *v); err != nil {
		return "", errors.WithStack(structs.ErrBadRequest("%s must be YYYY-MM-DD", name))
	}
	return *v, nil
}
