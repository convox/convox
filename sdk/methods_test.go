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

// TestSdkServiceScaleOverrideSet verifies ServiceScaleOverrideSet posts
// the expected URL + form params (active + optional ack_by).
func TestSdkServiceScaleOverrideSet(t *testing.T) {
	t.Run("active=true_with_ack_by", func(t *testing.T) {
		var (
			gotPath string
			gotForm map[string][]string
		)
		ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			require.NoError(t, r.ParseForm())
			gotForm = r.PostForm
			w.WriteHeader(http.StatusOK)
		}))
		defer ht.Close()

		c, err := sdk.New(ht.URL)
		require.NoError(t, err)

		require.NoError(t, c.ServiceScaleOverrideSet("app1", "web", true, "alice@example.com"))

		require.Equal(t, "/apps/app1/services/web/scale-override", gotPath)
		require.Equal(t, []string{"true"}, gotForm["active"])
		require.Equal(t, []string{"alice@example.com"}, gotForm["ack_by"])
	})

	t.Run("active=false_no_ack_by", func(t *testing.T) {
		var gotForm map[string][]string
		ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, r.ParseForm())
			gotForm = r.PostForm
			w.WriteHeader(http.StatusOK)
		}))
		defer ht.Close()

		c, err := sdk.New(ht.URL)
		require.NoError(t, err)
		require.NoError(t, c.ServiceScaleOverrideSet("app1", "web", false, ""))

		require.Equal(t, []string{"false"}, gotForm["active"])
		_, hasAckBy := gotForm["ack_by"]
		require.False(t, hasAckBy, "empty ackBy must NOT appear in the wire form")
	})
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

	require.Equal(t, []string{"500"}, got["monthly-cap-usd"], "monthly-cap-usd not propagated on the wire")
	require.Equal(t, []string{"80"}, got["alert-threshold-percent"], "alert-threshold-percent not propagated")
	require.Equal(t, []string{"alert-only"}, got["at-cap-action"], "at-cap-action not propagated")
	require.Equal(t, []string{"1.0"}, got["pricing-adjustment"], "pricing-adjustment not propagated")
}
