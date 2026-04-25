package sdk_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/stretchr/testify/require"
)

func TestInstanceKeyroll(t *testing.T) {
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(&structs.KeyPair{
			Name:       options.String("test"),
			PrivateKey: options.String("test"),
		})
	}))
	defer ht.Close()

	c, err := sdk.New(ht.URL)
	require.NoError(t, err)

	v, err := c.InstanceKeyroll()
	require.NoError(t, err)
	require.NotNil(t, v)
	require.Equal(t, "test", *v.Name)
	require.Equal(t, "test", *v.PrivateKey)
}

func TestInstanceKeyrollEmptyBody(t *testing.T) {
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ht.Close()

	c, err := sdk.New(ht.URL)
	require.NoError(t, err)

	v, err := c.InstanceKeyroll()
	require.NoError(t, err)
	require.NotNil(t, v)
}

// TestAppBudgetSetWirePath proves the budget-set form fields survive
// stdsdk.MarshalOptions. A prior version used *float64 which stdsdk
// silently drops, breaking the CLI → rack wire path end-to-end.
func TestAppBudgetSetWirePath(t *testing.T) {
	var got map[string][]string
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		got = r.PostForm
		w.WriteHeader(http.StatusOK)
	}))
	defer ht.Close()

	c, err := sdk.New(ht.URL)
	require.NoError(t, err)

	cap := "500"
	alert := 80
	action := "alert-only"
	pa := "1.0"

	require.NoError(t, c.AppBudgetSet("app1", structs.AppBudgetOptions{
		MonthlyCapUsd:         &cap,
		AlertThresholdPercent: &alert,
		AtCapAction:           &action,
		PricingAdjustment:     &pa,
	}, "tester"))

	require.Equal(t, []string{"500"}, got["monthly_cap_usd"], "monthly_cap_usd not propagated on the wire")
	require.Equal(t, []string{"80"}, got["alert_threshold_percent"], "alert_threshold_percent not propagated")
	require.Equal(t, []string{"alert-only"}, got["at_cap_action"], "at_cap_action not propagated")
	require.Equal(t, []string{"1.0"}, got["pricing_adjustment"], "pricing_adjustment not propagated")
}
