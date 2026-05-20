package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
)

type capStatus struct {
	AtCap                 bool
	AutoShutdown          bool
	KedaServiceSet        map[string]bool
	ArmedCountdownMinutes int
}

func budgetCapStatus(rack sdk.Interface, appName string, stderr io.Writer) (capStatus, error) {
	cs, ok := budgetCapStatusBase(rack, appName, stderr)
	if !ok {
		return cs, nil
	}
	if ss, e := rack.ServiceList(appName); e == nil {
		populateKedaServiceSet(&cs, ss)
	} else {
		fmt.Fprintf(stderr, "ns=cli_budget at=service-list-error err=%q\n", e)
	}
	return cs, nil
}

func budgetCapStatusWithServices(rack sdk.Interface, appName string, services structs.Services, stderr io.Writer) (capStatus, error) {
	cs, ok := budgetCapStatusBase(rack, appName, stderr)
	if !ok {
		return cs, nil
	}
	populateKedaServiceSet(&cs, services)
	return cs, nil
}

func budgetCapStatusBase(rack sdk.Interface, appName string, stderr io.Writer) (capStatus, bool) {
	cfg, state, err := rack.AppBudgetGet(appName)
	if err != nil {
		fmt.Fprintf(stderr, "ns=cli_budget at=fetch-error err=%q\n", err)
		return capStatus{}, false
	}
	if cfg == nil || state == nil || !state.CircuitBreakerTripped {
		return capStatus{}, false
	}
	cs := capStatus{
		AtCap:                 cfg.AtCapAction == structs.BudgetAtCapActionBlockNewDeploys || cfg.AtCapAction == structs.BudgetAtCapActionAutoShutdown,
		AutoShutdown:          cfg.AtCapAction == structs.BudgetAtCapActionAutoShutdown,
		ArmedCountdownMinutes: -1,
	}
	if !cs.AtCap {
		return capStatus{}, false
	}
	if !cs.AutoShutdown {
		return cs, true
	}
	shutdownState := safeShutdownStateGetFromBudgetStatus(rack, appName, stderr)
	if shutdownState != nil {
		if shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
			(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero()) {
			notifyMin := shutdownState.NotifyBeforeMinutes
			if notifyMin <= 0 {
				notifyMin = structs.BudgetDefaultNotifyBeforeMinutes
			}
			fireAt := shutdownState.ArmedAt.Add(time.Duration(notifyMin) * time.Minute)
			remaining := time.Until(fireAt)
			countdown := int(remaining.Minutes())
			if countdown < 1 {
				countdown = 1
			}
			cs.ArmedCountdownMinutes = countdown
		}
	}
	return cs, true
}

func safeShutdownStateGetFromBudgetStatus(rack sdk.Interface, app string, stderr io.Writer) (state *structs.AppBudgetShutdownState) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "ns=cli_budget at=shutdown-state-fetch-recover err=%v\n", r)
			state = nil
		}
	}()
	s, err := rack.AppBudgetShutdownStateGet(app)
	if err != nil {
		fmt.Fprintf(stderr, "ns=cli_budget at=shutdown-state-fetch-error err=%q\n", err)
		return nil
	}
	return s
}

func populateKedaServiceSet(cs *capStatus, services structs.Services) {
	cs.KedaServiceSet = make(map[string]bool, len(services))
	for i := range services {
		cs.KedaServiceSet[services[i].Name] = serviceHasKedaSurface(&services[i])
	}
}

func serviceHasKedaSurface(s *structs.Service) bool {
	return s != nil && s.Autoscale != nil && s.Autoscale.Enabled
}

func decorateStatusForBudgetCap(podStatus, serviceName string, cs capStatus) string {
	if !cs.AtCap {
		return podStatus
	}
	sub := capSubStateToken(serviceName, cs)
	if podStatus == "" {
		return sub
	}
	return podStatus + " " + sub
}

func capSubStateToken(serviceName string, cs capStatus) string {
	switch {
	case cs.AutoShutdown && cs.ArmedCountdownMinutes > 0:
		return fmt.Sprintf("armed-%dm", cs.ArmedCountdownMinutes)
	case cs.KedaServiceSet[serviceName]:
		return "at-cap-keda"
	case cs.AutoShutdown:
		return "at-cap-auto"
	default:
		return "at-cap"
	}
}

const kedaCapBypassBanner = "KEDA-managed services may scale despite block-new-deploys\n" +
	"(v1 limitation; auto-shutdown closes gap in 3.24.6 — see release notes)"
