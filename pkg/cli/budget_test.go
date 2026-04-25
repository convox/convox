package cli_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func fxAppBudget() *structs.AppBudget {
	return &structs.AppBudget{
		MonthlyCapUsd:         500,
		AlertThresholdPercent: 80,
		AtCapAction:           "alert-only",
		PricingAdjustment:     1.0,
	}
}

func fxAppBudgetState() *structs.AppBudgetState {
	return &structs.AppBudgetState{
		MonthStart:            time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		CurrentMonthSpendUsd:  123.45,
		CurrentMonthSpendAsOf: time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
	}
}

func TestBudgetShow(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetGet", "app1").Return(fxAppBudget(), fxAppBudgetState(), nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "monthly_cap_usd")
		require.Contains(t, res.Stdout, "500")
		require.Contains(t, res.Stdout, "alert-only")
		require.Contains(t, res.Stdout, "current_month_spend_usd")
		require.Contains(t, res.Stdout, "123.45")
	})
}

func TestBudgetShowNoBudget(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "no budget configured")
	})
}

func TestBudgetSetDefaults(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
			return opts.MonthlyCapUsd != nil && *opts.MonthlyCapUsd == "500" &&
				opts.AlertThresholdPercent != nil && *opts.AlertThresholdPercent == 80 &&
				opts.AtCapAction != nil && *opts.AtCapAction == "alert-only" &&
				opts.PricingAdjustment != nil && *opts.PricingAdjustment == "1"
		}), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 500", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "OK")
	})
}

func TestBudgetSetExplicit(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
			return opts.MonthlyCapUsd != nil && *opts.MonthlyCapUsd == "1000" &&
				opts.AlertThresholdPercent != nil && *opts.AlertThresholdPercent == 75 &&
				opts.AtCapAction != nil && *opts.AtCapAction == "block-new-deploys" &&
				opts.PricingAdjustment != nil && *opts.PricingAdjustment == "0.7"
		}), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 1000 --alert-at 75 --at-cap-action block-new-deploys --pricing-adjustment 0.7", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
	})
}

func TestBudgetSetRejectsNonNumericCap(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1 --monthly-cap abc", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "must be a number")
	})
}

func TestBudgetSetRejectsNonNumericAdjustment(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1 --monthly-cap 500 --pricing-adjustment xyz", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "must be a number")
	})
}

func TestBudgetSetMissingCap(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--monthly-cap")
	})
}

func TestBudgetSetInvalidAction(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1 --monthly-cap 500 --at-cap-action nope", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "at-cap-action")
	})
}

func TestBudgetClear(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetClear", "app1", mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget clear app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
	})
}

func TestBudgetResetForce(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetReset", "app1", mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget reset app1 --force", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
	})
}

// Non-interactive stdin without --force must error out rather than silently
// abort after an invisible prompt.
func TestBudgetResetNonInteractiveRequiresForce(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget reset app1", bytes.NewBufferString(""))
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--force")
	})
}

func TestBudgetResetError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetReset", "app1", mock.AnythingOfType("string")).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "budget reset app1 --force", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "err1")
	})
}

// silence unused import when fixtures in other files change
var _ = options.Int
