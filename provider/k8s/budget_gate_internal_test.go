package k8s

import (
	"strings"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
)

// Internal tests for requireCostTrackingForBudget and
// requireCostTrackingForManifestBudget. Both helpers are package-private
// (they read p.costTrackingEnabled() which is also private), so the tests
// live in package k8s rather than k8s_test.

func TestRequireCostTrackingForBudget_NoEnforcementBearing_AllowsThroughDisabledTracking(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "false")
	p := &Provider{}

	if err := p.requireCostTrackingForBudget(structs.AppBudgetOptions{}); err != nil {
		t.Errorf("empty options should pass with cost tracking disabled, got: %v", err)
	}

	pa := "0.7"
	if err := p.requireCostTrackingForBudget(structs.AppBudgetOptions{PricingAdjustment: &pa}); err != nil {
		t.Errorf("PricingAdjustment-only must pass with cost tracking disabled, got: %v", err)
	}
}

func TestRequireCostTrackingForBudget_EnforcementBearing_BlockedByDisabledTracking(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "false")
	p := &Provider{}

	cap := "500"
	pct := 80
	act := "auto-shutdown"
	cases := []structs.AppBudgetOptions{
		{MonthlyCapUsd: &cap},
		{AlertThresholdPercent: &pct},
		{AtCapAction: &act},
	}
	for _, opts := range cases {
		if err := p.requireCostTrackingForBudget(opts); err == nil {
			t.Errorf("enforcement-bearing options %+v should be rejected", opts)
		} else if !strings.Contains(err.Error(), "cost_tracking_enable") {
			t.Errorf("rejection message %q should mention cost_tracking_enable", err.Error())
		}
	}
}

func TestRequireCostTrackingForBudget_EnforcementBearing_AllowedWithEnabledTracking(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	p := &Provider{}

	cap := "500"
	if err := p.requireCostTrackingForBudget(structs.AppBudgetOptions{MonthlyCapUsd: &cap}); err != nil {
		t.Errorf("enforcement-bearing options should pass with cost tracking enabled, got: %v", err)
	}
}

func TestRequireCostTrackingForManifestBudget_NilManifestAllowed(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "false")
	p := &Provider{}
	if err := p.requireCostTrackingForManifestBudget(nil); err != nil {
		t.Errorf("nil manifest should pass: %v", err)
	}
}

func TestRequireCostTrackingForManifestBudget_NoBudgetAllowed(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "false")
	p := &Provider{}
	m := &manifest.Manifest{}
	if err := p.requireCostTrackingForManifestBudget(m); err != nil {
		t.Errorf("manifest without budget block should pass: %v", err)
	}
}

func TestRequireCostTrackingForManifestBudget_PricingAdjustmentOnlyAllowed(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "false")
	p := &Provider{}
	m := &manifest.Manifest{Budget: manifest.BudgetSettings{PricingAdjustment: 0.7}}
	if err := p.requireCostTrackingForManifestBudget(m); err != nil {
		t.Errorf("manifest with only PricingAdjustment should pass: %v", err)
	}
}

func TestRequireCostTrackingForManifestBudget_EnforcementBearing_Rejected(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "false")
	p := &Provider{}

	cases := []manifest.BudgetSettings{
		{MonthlyCapUsd: 500},
		{AlertThresholdPercent: 80},
		{AtCapAction: "auto-shutdown"},
	}
	for _, b := range cases {
		m := &manifest.Manifest{Budget: b}
		err := p.requireCostTrackingForManifestBudget(m)
		if err == nil {
			t.Errorf("manifest with %+v should be rejected", b)
			continue
		}
		if !strings.Contains(err.Error(), "cost_tracking_enable") {
			t.Errorf("rejection %q should mention cost_tracking_enable", err.Error())
		}
		var hErr *structs.HttpError
		if !asHttpError(err, &hErr) {
			t.Errorf("rejection should be *structs.HttpError")
		} else if hErr.Code() != 422 {
			t.Errorf("expected HTTP 422, got %d", hErr.Code())
		}
	}
}

func TestRequireCostTrackingForManifestBudget_EnforcementBearing_AllowedWithEnabledTracking(t *testing.T) {
	t.Setenv("COST_TRACKING_ENABLE", "true")
	p := &Provider{}
	m := &manifest.Manifest{Budget: manifest.BudgetSettings{MonthlyCapUsd: 500}}
	if err := p.requireCostTrackingForManifestBudget(m); err != nil {
		t.Errorf("manifest with cap should pass when cost tracking enabled, got: %v", err)
	}
}

// asHttpError unwraps to *structs.HttpError without pulling in errors.As's
// reflection cost in the hot test path; works against the helper's direct
// return shape (structs.ErrUnprocessable returns *HttpError).
func asHttpError(err error, target **structs.HttpError) bool {
	for err != nil {
		if h, ok := err.(*structs.HttpError); ok {
			*target = h
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
