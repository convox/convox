package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
)

type capStatus struct {
	AtCap  bool
	All    string
	Tokens map[string]string
}

func budgetCapStatus(rack sdk.Interface, appName string, stderr io.Writer) (capStatus, error) {
	cfg, state, err := rack.AppBudgetGet(appName)
	if err != nil {
		if !isRackVersionGated(err) {
			fmt.Fprintf(stderr, "ns=cli_budget at=fetch-error err=%q\n", err)
		}
		return capStatus{}, nil
	}
	if cfg == nil {
		return capStatus{}, nil
	}
	cs := capStatus{}
	if state != nil && state.CircuitBreakerTripped {
		cs.All = "at-cap"
	}
	if cfg.AtCapAction == structs.BudgetAtCapActionAutoShutdown {
		cs.Tokens = autoShutdownTokens(safeShutdownStateGetFromBudgetStatus(rack, appName, stderr), time.Now())
	}
	cs.AtCap = cs.All != "" || len(cs.Tokens) > 0
	return cs, nil
}

func autoShutdownTokens(s *structs.AppBudgetShutdownState, now time.Time) map[string]string {
	if s == nil || isSet(s.RestoredAt) || isSet(s.ExpiredAt) {
		return nil
	}
	tokens := map[string]string{}
	if !isSet(s.ShutdownAt) {
		if !isSet(s.ArmedAt) {
			return nil
		}
		notifyMin := s.NotifyBeforeMinutes
		if notifyMin <= 0 {
			notifyMin = structs.BudgetDefaultNotifyBeforeMinutes
		}
		countdown := int(s.ArmedAt.Add(time.Duration(notifyMin) * time.Minute).Sub(now).Minutes())
		if countdown < 1 {
			countdown = 1
		}
		for _, svc := range s.Services {
			tokens[svc.Name] = fmt.Sprintf("armed-%dm", countdown)
		}
		return tokens
	}
	for _, svc := range s.Services {
		switch {
		case !isSet(svc.ShutdownAt):
		case svc.KedaScaledObject != nil:
			tokens[svc.Name] = "at-cap-keda"
		default:
			tokens[svc.Name] = "at-cap-auto"
		}
	}
	return tokens
}

func isSet(t *time.Time) bool {
	return t != nil && !t.IsZero()
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

func serviceHasKedaSurface(s *structs.Service) bool {
	return s != nil && s.Autoscale != nil && s.Autoscale.Enabled
}

func decorateStatusForBudgetCap(podStatus, serviceName string, cs capStatus) string {
	sub := capSubStateToken(serviceName, cs)
	switch {
	case sub == "":
		return podStatus
	case podStatus == "":
		return sub
	default:
		return podStatus + " " + sub
	}
}

func capSubStateToken(serviceName string, cs capStatus) string {
	if t, ok := cs.Tokens[serviceName]; ok {
		return t
	}
	return cs.All
}

const kedaScalesAtCapBanner = "KEDA-managed services may scale despite block-new-deploys; auto-shutdown stops them at the cap"
