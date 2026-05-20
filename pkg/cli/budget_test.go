package cli_test

import (
	"bytes"
	"fmt"
	"os"
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
		require.Contains(t, res.Stdout, "monthly-cap-usd")
		require.Contains(t, res.Stdout, "500")
		require.Contains(t, res.Stdout, "alert-only")
		require.Contains(t, res.Stdout, "current-month-spend-usd")
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
		i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0.0}, nil)
		i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
			return opts.MonthlyCapUsd != nil && *opts.MonthlyCapUsd == "500" &&
				opts.AlertThresholdPercent != nil && *opts.AlertThresholdPercent == 80 &&
				opts.AtCapAction != nil && *opts.AtCapAction == "alert-only" &&
				opts.PricingAdjustment == nil
		}), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 500", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "OK")
	})
}

func TestBudgetSetExplicit(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0.0}, nil)
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

// TestBudgetSet_BelowMtdSpend_EmitsWarning verifies setting a cap below
// current MTD spend emits a non-blocking stderr warning.
func TestBudgetSet_BelowMtdSpend_EmitsWarning(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(&structs.AppCost{
			App:      "app1",
			SpendUsd: 200.0,
		}, nil)
		i.On("AppBudgetSet", "app1", mock.AnythingOfType("structs.AppBudgetOptions"), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 100", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		// Stderr captures both the WARNING line and the writer's terminal output.
		require.Contains(t, res.Stderr, "WARNING")
		require.Contains(t, res.Stderr, "100.00")
		require.Contains(t, res.Stderr, "200.00")
		require.Contains(t, res.Stdout, "OK")
	})
}

// TestBudgetSet_AboveMtdSpend_NoWarning — symmetric to above; cap above
// spend produces no warning.
func TestBudgetSet_AboveMtdSpend_NoWarning(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(&structs.AppCost{
			App:      "app1",
			SpendUsd: 50.0,
		}, nil)
		i.On("AppBudgetSet", "app1", mock.AnythingOfType("structs.AppBudgetOptions"), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 200", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.NotContains(t, res.Stderr, "WARNING")
	})
}

// TestBudgetSet_AppCostError_DoesNotBlock — a transient AppCost lookup
// failure must not block the budget-set call. The cap is still set; the
// warning is silently skipped.
func TestBudgetSet_AppCostError_DoesNotBlock(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(nil, fmt.Errorf("transient lookup failure"))
		i.On("AppBudgetSet", "app1", mock.AnythingOfType("structs.AppBudgetOptions"), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 200", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.NotContains(t, res.Stderr, "WARNING")
		require.Contains(t, res.Stdout, "OK")
	})
}

// TestBudgetReset_AckByFromEnv — legacy env-derivation path. Setting
// $CONVOX_ACTOR populates ack_by; CLI passes through unchanged. Locks the
// pre-existing CONVOX_ACTOR/USER/USERNAME fallback.
func TestBudgetReset_AckByFromEnv(t *testing.T) {
	prev := os.Getenv("CONVOX_ACTOR")
	defer os.Setenv("CONVOX_ACTOR", prev)
	require.NoError(t, os.Setenv("CONVOX_ACTOR", "alice"))

	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetReset", "app1", "alice").Return(nil)

		res, err := testExecute(e, "budget reset app1 --force", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.NotContains(t, res.Stderr, "DEPRECATED", "no explicit --ack-by → no deprecation warning")
		require.NotContains(t, res.Stderr, "deprecated", "no explicit --ack-by → no deprecation warning")
	})
}

// TestBudgetReset_ExplicitAckByFlag_PrintsDeprecationWarning — explicit flag
// triggers the deprecation warning on stderr.
func TestBudgetReset_ExplicitAckByFlag_PrintsDeprecationWarning(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetReset", "app1", "alice").Return(nil)

		res, err := testExecute(e, "budget reset app1 --force --ack-by alice", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stderr, "deprecated", "explicit --ack-by must emit deprecation warning")
		require.Contains(t, res.Stderr, "--ack-by", "deprecation warning must mention the flag name")
		require.Contains(t, res.Stderr, "3.25.0", "deprecation warning must cite the cliff version")
	})
}

// TestBudgetReset_DeprecationWarning_GoesToStderr verifies the deprecation
// warning goes to stderr, not stdout, so CI parsers are not affected.
func TestBudgetReset_DeprecationWarning_GoesToStderr(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetReset", "app1", "alice").Return(nil)

		res, err := testExecute(e, "budget reset app1 --force --ack-by alice", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)

		// WARNING text MUST appear on stderr, NOT stdout.
		require.Contains(t, res.Stderr, "WARNING", "deprecation warning lives on stderr")
		require.Contains(t, res.Stderr, "--ack-by")
		require.NotContains(t, res.Stdout, "WARNING", "deprecation warning must NOT leak onto stdout (CI parser regression guard)")
		require.NotContains(t, res.Stdout, "deprecated", "deprecation warning must NOT leak onto stdout")
	})
}

// TestBudgetSet_ExplicitAckByFlag_PrintsDeprecationWarning verifies
// --ack-by on budget set emits the stderr deprecation warning.
func TestBudgetSet_ExplicitAckByFlag_PrintsDeprecationWarning(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0.0}, nil)
		i.On("AppBudgetSet", "app1", mock.AnythingOfType("structs.AppBudgetOptions"), "alice").Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 500 --ack-by alice", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stderr, "deprecated")
		require.Contains(t, res.Stderr, "--ack-by")
	})
}

// TestBudgetClear_ExplicitAckByFlag_PrintsDeprecationWarning verifies
// --ack-by on budget clear emits the stderr deprecation warning.
func TestBudgetClear_ExplicitAckByFlag_PrintsDeprecationWarning(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetClear", "app1", "alice").Return(nil)

		res, err := testExecute(e, "budget clear app1 --ack-by alice", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stderr, "deprecated")
	})
}

// TestBudgetSet_AutoShutdownActionPrintsWarning verifies --at-cap-action=auto-shutdown
// emits a stderr warning.
func TestBudgetSet_AutoShutdownActionPrintsWarning(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0}, nil)
		i.On("AppBudgetSet", "app1", mock.AnythingOfType("structs.AppBudgetOptions"), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 500 --at-cap-action auto-shutdown", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stderr, "WARNING")
		require.Contains(t, res.Stderr, "auto-shutdown")
		require.Contains(t, res.Stderr, "simulate-shutdown")
	})
}

// TestBudgetReset_ForceClearCooldownFlag_PassesThroughToSDK verifies
// --force-clear-cooldown reaches the SDK.
func TestBudgetReset_ForceClearCooldownFlag_PassesThroughToSDK(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetResetWithOptions", "app1", mock.AnythingOfType("string"),
			mock.MatchedBy(func(opts structs.AppBudgetResetOptions) bool {
				return opts.ForceClearCooldown
			})).Return(nil)

		res, err := testExecute(e, "budget reset app1 --force --force-clear-cooldown", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
	})
}

// TestBudgetSimulateShutdown_OutputFormatMatchesSpec verifies CLI output
// includes section labels users script around.
func TestBudgetSimulateShutdown_OutputFormatMatchesSpec(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		i.On("AppBudgetSimulate", "app1").Return(&structs.AppBudgetSimulationResult{
			App:                 "app1",
			AtCapAction:         "auto-shutdown",
			WebhookUrl:          "https://hooks.example.com/budget",
			NotifyBeforeMinutes: 30,
			ShutdownGracePeriod: "5m0s",
			ShutdownOrder:       "largest-cost",
			RecoveryMode:        "auto-on-reset",
			Eligibility: []structs.AppBudgetSimulationEligibility{
				{Service: "api", Eligible: false, Reason: "in neverAutoShutdown"},
				{Service: "ml-batch", Eligible: true, Replicas: 3, CostUsdPerHour: 5.00},
			},
			WouldShutDownServices:        []string{"ml-batch"},
			WouldShutDownCount:           1,
			EstimatedCostSavedUsdPerHour: 5.00,
			SimulatedAt:                  now,
		}, nil)

		res, err := testExecute(e, "budget simulate-shutdown app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "Configuration:")
		require.Contains(t, res.Stdout, "auto-shutdown")
		require.Contains(t, res.Stdout, "Eligibility:")
		require.Contains(t, res.Stdout, "ml-batch: ELIGIBLE")
		require.Contains(t, res.Stdout, "api: EXEMPT")
		require.Contains(t, res.Stdout, "Estimated savings: $5.00/hr")
		require.Contains(t, res.Stdout, "SIMULATION COMPLETE")
	})
}

// TestBudgetDismissRecovery_Output verifies that
// `convox budget dismiss-recovery` emits one of three messages
// distinguishing dismissed / already-dismissed / no-banner states.
func TestBudgetDismissRecovery_Output(t *testing.T) {
	t.Run("dismissed", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("AppBudgetDismissRecoveryWithResult", "app1", mock.AnythingOfType("string")).
				Return(&structs.AppBudgetDismissRecoveryResult{App: "app1", Status: structs.BudgetDismissRecoveryStatusDismissed}, nil)

			res, err := testExecute(e, "budget dismiss-recovery app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
			require.Contains(t, res.Stdout, "Banner dismissed for app1")
		})
	})
	t.Run("already-dismissed", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("AppBudgetDismissRecoveryWithResult", "app1", mock.AnythingOfType("string")).
				Return(&structs.AppBudgetDismissRecoveryResult{App: "app1", Status: structs.BudgetDismissRecoveryStatusAlreadyDismissed}, nil)

			res, err := testExecute(e, "budget dismiss-recovery app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
			require.Contains(t, res.Stdout, "Banner already dismissed for app1")
		})
	})
	t.Run("no-banner", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("AppBudgetDismissRecoveryWithResult", "app1", mock.AnythingOfType("string")).
				Return(&structs.AppBudgetDismissRecoveryResult{App: "app1", Status: structs.BudgetDismissRecoveryStatusNoBanner}, nil)

			res, err := testExecute(e, "budget dismiss-recovery app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
			require.Contains(t, res.Stdout, "No recovery banner active for app1")
		})
	})
}

// TestBudgetSet_RejectsUnknownActionValue verifies unknown at-cap-action
// values are rejected while valid values (including auto-shutdown) pass.
func TestBudgetSet_RejectsUnknownActionValue(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1 --monthly-cap 500 --at-cap-action turbo-shutdown", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "at-cap-action")
	})
}

// TestBudgetCapRaise_HappyPath verifies budget cap raise is a partial-update
// alias for budget set --monthly-cap.
func TestBudgetCapRaise_HappyPath(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0.0}, nil)
		i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
			return opts.MonthlyCapUsd != nil && *opts.MonthlyCapUsd == "1000" &&
				opts.AlertThresholdPercent == nil &&
				opts.AtCapAction == nil &&
				opts.PricingAdjustment == nil
		}), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget cap raise app1 --monthly-cap-usd 1000", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "OK")
	})
}

// TestBudgetCapRaise_AcceptsMonthlyCapAlias verifies --monthly-cap is accepted
// as an alias for --monthly-cap-usd, and the canonical flag wins when both set.
func TestBudgetCapRaise_AcceptsMonthlyCapAlias(t *testing.T) {
	t.Run("alias-only accepted with canonical-equivalent behavior", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0.0}, nil)
			i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
				return opts.MonthlyCapUsd != nil && *opts.MonthlyCapUsd == "200"
			}), mock.AnythingOfType("string")).Return(nil)

			res, err := testExecute(e, "budget cap raise app1 --monthly-cap 200", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
			require.Contains(t, res.Stdout, "OK")
		})
	})

	t.Run("canonical wins when both flags provided", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0.0}, nil)
			i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
				return opts.MonthlyCapUsd != nil && *opts.MonthlyCapUsd == "300"
			}), mock.AnythingOfType("string")).Return(nil)

			res, err := testExecute(e, "budget cap raise app1 --monthly-cap-usd 300 --monthly-cap 200", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		})
	})
}

// TestBudgetCapRaise_MissingFlag verifies --monthly-cap-usd is required.
func TestBudgetCapRaise_MissingFlag(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget cap raise app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--monthly-cap-usd")
	})
}

// TestBudgetCapRaise_RejectsNonNumericCap verifies non-numeric --monthly-cap-usd
// is rejected.
func TestBudgetCapRaise_RejectsNonNumericCap(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget cap raise app1 --monthly-cap-usd abc", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "must be a number")
	})
}

// TestBudgetCapRaise_BelowMtdSpend_EmitsWarning verifies cap below current
// MTD spend emits a non-blocking warning.
func TestBudgetCapRaise_BelowMtdSpend_EmitsWarning(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(&structs.AppCost{
			App:      "app1",
			SpendUsd: 750.0,
		}, nil)
		i.On("AppBudgetSet", "app1", mock.AnythingOfType("structs.AppBudgetOptions"), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget cap raise app1 --monthly-cap-usd 500", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stderr, "WARNING")
		require.Contains(t, res.Stderr, "500.00")
		require.Contains(t, res.Stderr, "750.00")
		require.Contains(t, res.Stdout, "OK")
	})
}

// TestBudgetCapRaise_AppCostError_DoesNotBlock verifies transient AppCost
// failure does not block the cap-raise call.
func TestBudgetCapRaise_AppCostError_DoesNotBlock(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(nil, fmt.Errorf("transient lookup failure"))
		i.On("AppBudgetSet", "app1", mock.AnythingOfType("structs.AppBudgetOptions"), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget cap raise app1 --monthly-cap-usd 800", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.NotContains(t, res.Stderr, "WARNING")
		require.Contains(t, res.Stdout, "OK")
	})
}

// TestBudgetSimulateShutdown_OutputDoesNotReferenceUnimplementedCommand
// verifies simulate-shutdown output does not cite unregistered commands.
func TestBudgetSimulateShutdown_OutputDoesNotReferenceUnimplementedCommand(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		i.On("AppBudgetSimulate", "app1").Return(&structs.AppBudgetSimulationResult{
			App:                          "app1",
			AtCapAction:                  "auto-shutdown",
			WebhookUrl:                   "https://hooks.example.com/budget",
			NotifyBeforeMinutes:          30,
			ShutdownGracePeriod:          "5m0s",
			ShutdownOrder:                "largest-cost",
			RecoveryMode:                 "auto-on-reset",
			Eligibility:                  []structs.AppBudgetSimulationEligibility{},
			WouldShutDownServices:        []string{},
			WouldShutDownCount:           0,
			EstimatedCostSavedUsdPerHour: 0,
			SimulatedAt:                  now,
		}, nil)

		res, err := testExecute(e, "budget simulate-shutdown app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.NotContains(t, res.Stdout, "convox events", "simulate-shutdown must not cite unimplemented `convox events` subcommand")
		require.NotContains(t, res.Stdout, "events --rack", "simulate-shutdown must not cite `events --rack` flag")
		// Verify the replacement text is present so users have a real surface to look at.
		require.Contains(t, res.Stdout, "atCapWebhookUrl", "simulate-shutdown must point at the real webhook surface")
		require.Contains(t, res.Stdout, "rack log aggregation", "simulate-shutdown must also point at log aggregation as fallback")
	})
}

// TestBudgetShow_FailedBanner_RendersReason verifies the FAILED banner
// renders FailureReason from persisted state.
func TestBudgetShow_FailedBanner_RendersReason(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		armed := now.Add(-1 * time.Hour)
		shut := now.Add(-30 * time.Minute)
		i.On("AppBudgetGet", "app1").Return(fxAppBudget(), fxAppBudgetState(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(&structs.AppBudgetShutdownState{
			SchemaVersion:             1,
			ArmedAt:                   &armed,
			ShutdownAt:                &shut,
			RecoveryMode:              "auto-on-reset",
			ShutdownOrder:             "largest-cost",
			ShutdownTickId:            "tick-failed-test",
			EligibleServiceCount:      1,
			Services:                  []structs.AppBudgetShutdownStateService{{Name: "ml-batch"}},
			FailedNotificationFiredAt: &now,
			FailureReason:             structs.BudgetShutdownReasonK8sApiFailure,
		}, nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "[FAILED]", "FAILED banner sentinel must appear")
		require.Contains(t, res.Stdout, "Auto-shutdown FAILED for app1", "FAILED banner header text")
		require.Contains(t, res.Stdout, "Reason: k8s-api-failure", "Reason: <failureReason> must be rendered")
		require.Contains(t, res.Stdout, "convox budget reset app1", "FAILED banner must still cite the recovery command")
	})
}

// TestBudgetShow_FailedBanner_NoReason_FallsBackToLegacy verifies empty
// FailureReason falls back to legacy text instead of rendering "Reason: .".
func TestBudgetShow_FailedBanner_NoReason_FallsBackToLegacy(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		armed := now.Add(-1 * time.Hour)
		shut := now.Add(-30 * time.Minute)
		i.On("AppBudgetGet", "app1").Return(fxAppBudget(), fxAppBudgetState(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(&structs.AppBudgetShutdownState{
			SchemaVersion:             1,
			ArmedAt:                   &armed,
			ShutdownAt:                &shut,
			RecoveryMode:              "auto-on-reset",
			ShutdownOrder:             "largest-cost",
			ShutdownTickId:            "tick-failed-no-reason",
			EligibleServiceCount:      1,
			Services:                  []structs.AppBudgetShutdownStateService{{Name: "ml-batch"}},
			FailedNotificationFiredAt: &now,
			// FailureReason intentionally empty — defensive cross-version path.
		}, nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "[FAILED]", "FAILED banner sentinel still appears")
		require.Contains(t, res.Stdout, "Auto-shutdown FAILED for app1", "legacy text retained when reason absent")

		require.NotContains(t, res.Stdout, "Reason: .", "must not render empty Reason: . token when FailureReason is empty")
		require.NotContains(t, res.Stdout, "Reason: ", "must not render Reason: prefix at all when FailureReason is empty")
	})
}

// TestBudgetShow_ArmedBanner_RendersFireAt verifies the ARMED banner
// computes fireAt from ArmedAt + notifyBeforeMinutes.
func TestBudgetShow_ArmedBanner_RendersFireAt(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		armed := now.Add(-10 * time.Minute)
		i.On("AppBudgetGet", "app1").Return(fxAppBudget(), fxAppBudgetState(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(&structs.AppBudgetShutdownState{
			SchemaVersion:            1,
			ArmedAt:                  &armed,
			RecoveryMode:             "auto-on-reset",
			ShutdownOrder:            "largest-cost",
			ShutdownTickId:           "tick-armed-test",
			EligibleServiceCount:     1,
			Services:                 []structs.AppBudgetShutdownStateService{{Name: "ml-batch"}},
			ArmedNotificationFiredAt: &armed,
		}, nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "[ARMED]", "ARMED banner sentinel must appear")
		require.Contains(t, res.Stdout, "Auto-shutdown ARMED for app1", "ARMED banner header text")
		// fireAt = armed + 30m default notifyBeforeMinutes
		expectedFireAt := armed.Add(30 * time.Minute).UTC().Format("2006-01-02T15:04:05Z")
		require.Contains(t, res.Stdout, expectedFireAt, "ARMED banner must render computed fireAt = armedAt + notifyBeforeMinutes")
		require.Contains(t, res.Stdout, "convox budget cap raise --monthly-cap-usd <higher> app1", "ARMED banner must cite cap raise command with app placeholder")
		require.Contains(t, res.Stdout, "convox budget reset app1", "ARMED banner must cite reset command")
	})
}

// TestBudgetShow_ActiveBanner_RendersServiceCount verifies the ACTIVE banner
// renders service count and shutdown timestamp.
func TestBudgetShow_ActiveBanner_RendersServiceCount(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		armed := now.Add(-1 * time.Hour)
		shut := now.Add(-30 * time.Minute)
		i.On("AppBudgetGet", "app1").Return(fxAppBudget(), fxAppBudgetState(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(&structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			ShutdownAt:           &shut,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-active-test",
			EligibleServiceCount: 3,
			Services: []structs.AppBudgetShutdownStateService{
				{Name: "ml-batch"},
				{Name: "ml-train"},
				{Name: "ml-infer"},
			},
			ArmedNotificationFiredAt: &armed,
			FiredNotificationFiredAt: &shut,
		}, nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "[ACTIVE]", "ACTIVE banner sentinel must appear")
		require.Contains(t, res.Stdout, "Auto-shutdown ACTIVE for app1", "ACTIVE banner header text")
		require.Contains(t, res.Stdout, "3 services scaled to 0", "ACTIVE banner must render len(Services) count")
		require.Contains(t, res.Stdout, shut.UTC().Format("2006-01-02T15:04:05Z"), "ACTIVE banner must render shutdown timestamp")
		require.Contains(t, res.Stdout, "convox budget reset app1", "ACTIVE banner must cite restore command")
	})
}

// TestBudgetShow_RecoveredBanner_RendersWithAndWithoutFlapWindow verifies
// RECOVERED banner with and without cooldown text.
func TestBudgetShow_RecoveredBanner_RendersWithAndWithoutFlapWindow(t *testing.T) {
	now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
	armed := now.Add(-2 * time.Hour)
	shut := now.Add(-1 * time.Hour)
	restored := now.Add(-15 * time.Minute)
	flap := now.Add(23*time.Hour + 45*time.Minute)

	cases := []struct {
		name              string
		flapSuppressed    *time.Time
		expectCooldownTxt bool
	}{
		{name: "no_flap_window", flapSuppressed: nil, expectCooldownTxt: false},
		{name: "with_flap_window", flapSuppressed: &flap, expectCooldownTxt: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("AppBudgetGet", "app1").Return(fxAppBudget(), fxAppBudgetState(), nil)
				i.On("AppBudgetShutdownStateGet", "app1").Return(&structs.AppBudgetShutdownState{
					SchemaVersion:        1,
					ArmedAt:              &armed,
					ShutdownAt:           &shut,
					RestoredAt:           &restored,
					FlapSuppressedUntil:  tc.flapSuppressed,
					RecoveryMode:         "auto-on-reset",
					ShutdownOrder:        "largest-cost",
					ShutdownTickId:       "tick-recovered-test",
					EligibleServiceCount: 1,
					Services:             []structs.AppBudgetShutdownStateService{{Name: "ml-batch"}},
				}, nil)

				res, err := testExecute(e, "budget show app1", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
				require.Contains(t, res.Stdout, "[RECOVERED]", "RECOVERED banner sentinel must appear")
				require.Contains(t, res.Stdout, "Auto-shutdown RECOVERED for app1", "RECOVERED banner header text")
				require.Contains(t, res.Stdout, restored.UTC().Format("2006-01-02T15:04:05Z"), "RECOVERED banner must render restoredAt")
				require.Contains(t, res.Stdout, "convox budget dismiss-recovery app1", "RECOVERED banner must cite dismiss command")
				if tc.expectCooldownTxt {
					require.Contains(t, res.Stdout, "Cooldown until", "RECOVERED banner must render cooldown text when FlapSuppressedUntil set")
					require.Contains(t, res.Stdout, flap.UTC().Format("2006-01-02T15:04:05Z"), "cooldown timestamp must render")
				} else {
					require.NotContains(t, res.Stdout, "Cooldown until", "RECOVERED banner must omit cooldown text when FlapSuppressedUntil nil")
				}
			})
		})
	}
}

// TestBudgetShow_RecoveredOverridesFailed_AfterManualRecovery verifies
// RECOVERED takes precedence over FAILED when RestoredAt is set.
func TestBudgetShow_RecoveredOverridesFailed_AfterManualRecovery(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		armed := now.Add(-2 * time.Hour)
		shut := now.Add(-1 * time.Hour)
		failed := now.Add(-50 * time.Minute)
		restored := now.Add(-10 * time.Minute)
		i.On("AppBudgetGet", "app1").Return(fxAppBudget(), fxAppBudgetState(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(&structs.AppBudgetShutdownState{
			SchemaVersion:             1,
			ArmedAt:                   &armed,
			ShutdownAt:                &shut,
			RestoredAt:                &restored,
			RecoveryMode:              "auto-on-reset",
			ShutdownOrder:             "largest-cost",
			ShutdownTickId:            "tick-recovered-from-failed",
			EligibleServiceCount:      1,
			Services:                  []structs.AppBudgetShutdownStateService{{Name: "ml-batch"}},
			FailedNotificationFiredAt: &failed,
			FailureReason:             structs.BudgetShutdownReasonK8sApiFailure,
		}, nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "[RECOVERED]", "RECOVERED must take precedence over FAILED after manual recovery")
		require.NotContains(t, res.Stdout, "[FAILED]", "FAILED banner must NOT render when RestoredAt is set")
		require.NotContains(t, res.Stdout, "Auto-shutdown FAILED", "FAILED header text must NOT render alongside RECOVERED")
	})
}

// TestBudgetShow_BannerHonorsNotifyBeforeMinutes verifies ARMED banner uses
// persisted NotifyBeforeMinutes, falling back to 30m default when zero.
func TestBudgetShow_BannerHonorsNotifyBeforeMinutes(t *testing.T) {
	cases := []struct {
		name             string
		persistedNotify  int
		expectNotifyMins int
	}{
		{name: "explicit_60_minutes", persistedNotify: 60, expectNotifyMins: 60},
		{name: "explicit_5_minutes", persistedNotify: 5, expectNotifyMins: 5},
		{name: "zero_falls_back_to_default", persistedNotify: 0, expectNotifyMins: 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
				armed := now.Add(-1 * time.Minute) // freshly armed
				i.On("AppBudgetGet", "app1").Return(fxAppBudget(), fxAppBudgetState(), nil)
				i.On("AppBudgetShutdownStateGet", "app1").Return(&structs.AppBudgetShutdownState{
					SchemaVersion:            1,
					ArmedAt:                  &armed,
					NotifyBeforeMinutes:      tc.persistedNotify,
					RecoveryMode:             "auto-on-reset",
					ShutdownOrder:            "largest-cost",
					ShutdownTickId:           "tick-notify-test",
					EligibleServiceCount:     1,
					Services:                 []structs.AppBudgetShutdownStateService{{Name: "ml-batch"}},
					ArmedNotificationFiredAt: &armed,
				}, nil)

				res, err := testExecute(e, "budget show app1", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
				expectedFireAt := armed.Add(time.Duration(tc.expectNotifyMins) * time.Minute).UTC().Format("2006-01-02T15:04:05Z")
				require.Contains(t, res.Stdout, expectedFireAt,
					"ARMED banner must compute fireAt with persisted NotifyBeforeMinutes=%d (expected %d-minute window)",
					tc.persistedNotify, tc.expectNotifyMins)
			})
		})
	}
}

// Test 1: TestBudgetSet_MonthlyCapOnly_Accepted verifies --monthly-cap alone
// leaves PricingAdjustment nil (partial-merge).
func TestBudgetSet_MonthlyCapOnly_Accepted(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0.0}, nil)
		i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
			return opts.MonthlyCapUsd != nil && *opts.MonthlyCapUsd == "500" &&
				opts.AlertThresholdPercent != nil && *opts.AlertThresholdPercent == 80 &&
				opts.AtCapAction != nil && *opts.AtCapAction == "alert-only" &&
				opts.PricingAdjustment == nil
		}), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 500", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "OK")
	})
}

// Test 2: TestBudgetSet_PricingAdjustmentOnly_Accepted verifies pricing-only
// path sends only PricingAdjustment; AppCost must not be called.
func TestBudgetSet_PricingAdjustmentOnly_Accepted(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
			return opts.MonthlyCapUsd == nil &&
				opts.AlertThresholdPercent == nil &&
				opts.AtCapAction == nil &&
				opts.PricingAdjustment != nil && *opts.PricingAdjustment == "0.7"
		}), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --pricing-adjustment 0.7", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "OK")
		i.AssertNotCalled(t, "AppCost", "app1")
	})
}

// Test 3: TestBudgetSet_CapAndPricingAdjustment_Accepted verifies combined
// --monthly-cap and --pricing-adjustment populates all fields.
func TestBudgetSet_CapAndPricingAdjustment_Accepted(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0.0}, nil)
		i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
			return opts.MonthlyCapUsd != nil && *opts.MonthlyCapUsd == "500" &&
				opts.AlertThresholdPercent != nil && *opts.AlertThresholdPercent == 80 &&
				opts.AtCapAction != nil && *opts.AtCapAction == "alert-only" &&
				opts.PricingAdjustment != nil && *opts.PricingAdjustment == "0.7"
		}), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --monthly-cap 500 --pricing-adjustment 0.7", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "OK")
	})
}

// Test 4: TestBudgetSet_AlertAtWithoutCap_Rejected verifies --alert-at
// without --monthly-cap is rejected.
func TestBudgetSet_AlertAtWithoutCap_Rejected(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1 --alert-at 80", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--alert-at")
		require.Contains(t, res.Stderr, "require --monthly-cap")
		i.AssertNotCalled(t, "AppBudgetSet", mock.Anything, mock.Anything, mock.Anything)
	})
}

// Test 5: TestBudgetSet_AtCapActionWithoutCap_Rejected verifies
// --at-cap-action without --monthly-cap is rejected.
func TestBudgetSet_AtCapActionWithoutCap_Rejected(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1 --at-cap-action auto-shutdown", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--at-cap-action")
		require.Contains(t, res.Stderr, "require --monthly-cap")
		i.AssertNotCalled(t, "AppBudgetSet", mock.Anything, mock.Anything, mock.Anything)
	})
}

// Test 6: TestBudgetSet_AlertAtWithPricingAdjustment_Rejected verifies
// --alert-at with --pricing-adjustment still requires --monthly-cap.
func TestBudgetSet_AlertAtWithPricingAdjustment_Rejected(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1 --pricing-adjustment 0.7 --alert-at 80", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "require --monthly-cap")
		i.AssertNotCalled(t, "AppBudgetSet", mock.Anything, mock.Anything, mock.Anything)
	})
}

// Test 7: TestBudgetSet_NoFlags_Rejected verifies no flags produces an error.
func TestBudgetSet_NoFlags_Rejected(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--monthly-cap or --pricing-adjustment is required")
		i.AssertNotCalled(t, "AppBudgetSet", mock.Anything, mock.Anything, mock.Anything)
	})
}

// Test 8: TestBudgetSet_PricingAdjustmentOnly_RejectsNonNumeric verifies
// non-numeric pricing adjustment is rejected.
func TestBudgetSet_PricingAdjustmentOnly_RejectsNonNumeric(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1 --pricing-adjustment xyz", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "must be a number")
		i.AssertNotCalled(t, "AppBudgetSet", mock.Anything, mock.Anything, mock.Anything)
	})
}

// Test 9: TestBudgetSet_PricingAdjustmentOnly_Idempotent verifies repeated
// pricing-only calls produce the same options shape.
func TestBudgetSet_PricingAdjustmentOnly_Idempotent(t *testing.T) {
	matchPricingOnly := func(opts structs.AppBudgetOptions) bool {
		return opts.MonthlyCapUsd == nil &&
			opts.AlertThresholdPercent == nil &&
			opts.AtCapAction == nil &&
			opts.PricingAdjustment != nil && *opts.PricingAdjustment == "0.7"
	}

	for iter := 0; iter < 2; iter++ {
		testClient(t, func(e *cli.Engine, mockI *mocksdk.Interface) {
			mockI.On("AppBudgetSet", "app1", mock.MatchedBy(matchPricingOnly), mock.AnythingOfType("string")).Return(nil)

			res, err := testExecute(e, "budget set app1 --pricing-adjustment 0.7", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code, "iteration %d stderr: %s", iter, res.Stderr)
			require.Contains(t, res.Stdout, "OK", "iteration %d", iter)
		})
	}
}

// Test 10: TestBudgetSet_PricingAdjustmentOnly_OmissionPreservesPrior verifies
// pricing-only update leaves MonthlyCapUsd nil for server-side merge.
func TestBudgetSet_PricingAdjustmentOnly_OmissionPreservesPrior(t *testing.T) {
	t.Run("first set populates cap and pricing", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("AppCost", "app1").Return(&structs.AppCost{App: "app1", SpendUsd: 0.0}, nil)
			i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
				return opts.MonthlyCapUsd != nil && *opts.MonthlyCapUsd == "500" &&
					opts.PricingAdjustment != nil && *opts.PricingAdjustment == "0.5"
			}), mock.AnythingOfType("string")).Return(nil)

			res, err := testExecute(e, "budget set app1 --monthly-cap 500 --pricing-adjustment 0.5", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		})
	})

	t.Run("second set with only pricing leaves cap pointer nil", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
				return opts.MonthlyCapUsd == nil &&
					opts.AlertThresholdPercent == nil &&
					opts.AtCapAction == nil &&
					opts.PricingAdjustment != nil && *opts.PricingAdjustment == "0.7"
			}), mock.AnythingOfType("string")).Return(nil)

			res, err := testExecute(e, "budget set app1 --pricing-adjustment 0.7", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
			i.AssertNotCalled(t, "AppCost", "app1")
		})
	})
}

// Test 11: TestBudgetSet_PricingAdjustmentExplicitZero_Accepted verifies
// explicit zero is accepted.
func TestBudgetSet_PricingAdjustmentExplicitZero_Accepted(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetSet", "app1", mock.MatchedBy(func(opts structs.AppBudgetOptions) bool {
			return opts.MonthlyCapUsd == nil &&
				opts.PricingAdjustment != nil && *opts.PricingAdjustment == "0"
		}), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --pricing-adjustment 0", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "OK")
	})
}

// Test 12: TestBudgetSet_PricingAdjustmentInfNaN_Rejected verifies Inf and
// NaN are rejected.
func TestBudgetSet_PricingAdjustmentInfNaN_Rejected(t *testing.T) {
	for _, tv := range []string{"Inf", "NaN"} {
		t.Run(tv, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				res, err := testExecute(e, "budget set app1 --pricing-adjustment "+tv, nil)
				require.NoError(t, err)
				require.Equal(t, 1, res.Code)
				require.Contains(t, res.Stderr, "must be a finite number")
				i.AssertNotCalled(t, "AppBudgetSet", mock.Anything, mock.Anything, mock.Anything)
			})
		})
	}
}

// Test 13: TestBudgetSet_PricingAdjustmentOnly_NoMtdWarning verifies
// AppCost is not called and the misleading MTD warning is suppressed on
// the pricing-only path.
func TestBudgetSet_PricingAdjustmentOnly_NoMtdWarning(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// AppCost intentionally not mocked.
		i.On("AppBudgetSet", "app1", mock.AnythingOfType("structs.AppBudgetOptions"), mock.AnythingOfType("string")).Return(nil)

		res, err := testExecute(e, "budget set app1 --pricing-adjustment 0.7", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.NotContains(t, res.Stderr, "--monthly-cap=$0.00",
			"MTD warning must NOT cite a misleading $0.00 cap on the pricing-only path")
		require.NotContains(t, res.Stderr, "below current month-to-date spend",
			"MTD warning must be suppressed entirely on the pricing-only path")
		i.AssertNotCalled(t, "AppCost", "app1")
	})
}

// Test 14: TestBudgetSet_NoFlags_ErrorMessageContainsMonthlyCapSubstring
// ensures "--monthly-cap" substring survives rewording (CI grep contract).
func TestBudgetSet_NoFlags_ErrorMessageContainsMonthlyCapSubstring(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "budget set app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		// Substring guard — wording can change around it, but the literal
		// "--monthly-cap" must always be present (CI grep contract).
		require.Contains(t, res.Stderr, "--monthly-cap")
	})
}

// TestSimulateShutdown_LowRateAsEmDash verifies rates below $0.001/hr
// render as em-dash with disambiguation footnote.
func TestSimulateShutdown_LowRateAsEmDash(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		i.On("AppBudgetSimulate", "app1").Return(&structs.AppBudgetSimulationResult{
			App:                          "app1",
			AtCapAction:                  "auto-shutdown",
			WebhookUrl:                   "https://hooks.example.com/budget",
			NotifyBeforeMinutes:          30,
			ShutdownGracePeriod:          "5m0s",
			ShutdownOrder:                "largest-cost",
			RecoveryMode:                 "auto-on-reset",
			Eligibility:                  []structs.AppBudgetSimulationEligibility{{Service: "web", Eligible: true, Replicas: 1, CostUsdPerHour: 0.0005}},
			WouldShutDownServices:        []string{"web"},
			WouldShutDownCount:           1,
			EstimatedCostSavedUsdPerHour: 0.0005,
			SimulatedAt:                  now,
		}, nil)

		res, err := testExecute(e, "budget simulate-shutdown app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "cost=—/hr",
			"low-rate Eligible row must render the em-dash, not $0.00")
		require.Contains(t, res.Stdout, "low-spend rates rounded to —",
			"footnote must appear when at least one row used the em-dash")
	})
}

// TestSimulateShutdown_NoFootnoteWhenAllAboveThreshold verifies no
// footnote appears when all rows render normally.
func TestSimulateShutdown_NoFootnoteWhenAllAboveThreshold(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		i.On("AppBudgetSimulate", "app1").Return(&structs.AppBudgetSimulationResult{
			App:                 "app1",
			AtCapAction:         "auto-shutdown",
			WebhookUrl:          "https://hooks.example.com/budget",
			NotifyBeforeMinutes: 30,
			ShutdownGracePeriod: "5m0s",
			ShutdownOrder:       "largest-cost",
			RecoveryMode:        "auto-on-reset",
			Eligibility: []structs.AppBudgetSimulationEligibility{
				{Service: "web", Eligible: true, Replicas: 2, CostUsdPerHour: 1.50},
				{Service: "trainer", Eligible: true, Replicas: 1, CostUsdPerHour: 2.34},
			},
			WouldShutDownServices:        []string{"web", "trainer"},
			WouldShutDownCount:           2,
			EstimatedCostSavedUsdPerHour: 3.84,
			SimulatedAt:                  now,
		}, nil)

		res, err := testExecute(e, "budget simulate-shutdown app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.NotContains(t, res.Stdout, "low-spend rates rounded to —",
			"footnote must NOT appear when all rows render normally")
		require.Contains(t, res.Stdout, "cost=$1.50/hr")
		require.Contains(t, res.Stdout, "cost=$2.34/hr")
	})
}

// TestSimulateShutdown_ExemptServicesUnaffected verifies exempt rows are
// unchanged and only eligible rows use em-dash format.
func TestSimulateShutdown_ExemptServicesUnaffected(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Date(2026, 4, 25, 14, 0, 0, 0, time.UTC)
		i.On("AppBudgetSimulate", "app1").Return(&structs.AppBudgetSimulationResult{
			App:                 "app1",
			AtCapAction:         "auto-shutdown",
			WebhookUrl:          "https://hooks.example.com/budget",
			NotifyBeforeMinutes: 30,
			ShutdownGracePeriod: "5m0s",
			ShutdownOrder:       "largest-cost",
			RecoveryMode:        "auto-on-reset",
			Eligibility: []structs.AppBudgetSimulationEligibility{
				{Service: "api", Eligible: false, Reason: "in neverAutoShutdown"},
				{Service: "trainer", Eligible: true, Replicas: 1, CostUsdPerHour: 0.0008},
			},
			WouldShutDownServices:        []string{"trainer"},
			WouldShutDownCount:           1,
			EstimatedCostSavedUsdPerHour: 0.0008,
			SimulatedAt:                  now,
		}, nil)

		res, err := testExecute(e, "budget simulate-shutdown app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "api: EXEMPT (in neverAutoShutdown)",
			"Exempt row format must be unchanged")
		require.Contains(t, res.Stdout, "trainer: ELIGIBLE -- replicas=1, cost=—/hr",
			"low-rate Eligible row picks up em-dash")
		require.Contains(t, res.Stdout, "low-spend rates rounded to —",
			"footnote appears once due to Eligible row em-dash")
	})
}
