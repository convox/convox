package k8s_test

import (
	"testing"
	"time"

	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Per-URL webhook timeout tests covering the parser branches in isolation
// (no Provider, no informer). EventSend-side integration tests live in
// event_test.go (TestWebhookConfigmapJsonParseCache, TestWebhookPerUrlTimeout,
// TestParseFallsThroughOnInvalidJSON, TestSkipsEntryWithEmptyURL).

func TestParseWebhookEntry_PlainURL_DefaultTimeout(t *testing.T) {
	entry, skip := k8s.ParseWebhookEntryForTest("slack_alerts", "https://hooks.slack.com/services/T0/B0/key")
	require.False(t, skip, "plain URL must not be skipped")
	assert.Equal(t, "https://hooks.slack.com/services/T0/B0/key", entry.URL)
	assert.Equal(t, k8s.DefaultWebhookTimeoutForTest(), entry.Timeout,
		"plain URL must inherit defaultWebhookTimeout (30s)")
}

func TestParseWebhookEntry_JSONForm_HonorsTimeout(t *testing.T) {
	entry, skip := k8s.ParseWebhookEntryForTest("audit_internal",
		`{"url":"https://audit.internal.corp/v1/events","timeout":"60s"}`)
	require.False(t, skip)
	assert.Equal(t, "https://audit.internal.corp/v1/events", entry.URL)
	assert.Equal(t, 60*time.Second, entry.Timeout, "JSON timeout=60s must round-trip")
}

func TestParseWebhookEntry_JSONForm_FastTimeout(t *testing.T) {
	entry, skip := k8s.ParseWebhookEntryForTest("slack_alerts",
		`{"url":"https://hooks.slack.com/x","timeout":"5s"}`)
	require.False(t, skip)
	assert.Equal(t, 5*time.Second, entry.Timeout)
}

func TestParseWebhookEntry_JSONForm_AbsentTimeout_UsesDefault(t *testing.T) {
	entry, skip := k8s.ParseWebhookEntryForTest("hook", `{"url":"https://example.com/x"}`)
	require.False(t, skip)
	assert.Equal(t, k8s.DefaultWebhookTimeoutForTest(), entry.Timeout,
		"absent timeout in JSON form must fall back to default")
}

// Anti-trap: empty URL in JSON form must skip, NOT fall through to
// plain-URL semantics. If the parser fell through, dispatch would attempt
// to POST to the literal JSON-object string as a URL, corrupting wire
// behavior.
func TestParseWebhookEntry_JSONForm_EmptyURL_Skips_NotPlainURLFallthrough(t *testing.T) {
	raws := []string{
		`{"url":"","timeout":"5s"}`,
		`{"url":"   ","timeout":"5s"}`,
		`{"timeout":"5s"}`,
	}
	for _, raw := range raws {
		t.Run(raw, func(t *testing.T) {
			entry, skip := k8s.ParseWebhookEntryForTest("name", raw)
			assert.True(t, skip, "empty URL in JSON must skip")
			assert.Empty(t, entry.URL,
				"skipped JSON entries must NOT have raw value treated as URL")
		})
	}
}

func TestParseWebhookEntry_JSONForm_InvalidJSON_Skips(t *testing.T) {
	raws := []string{
		`{this is not json`,
		`{`,
		`{"url":}`,
	}
	for _, raw := range raws {
		t.Run(raw, func(t *testing.T) {
			_, skip := k8s.ParseWebhookEntryForTest("name", raw)
			assert.True(t, skip, "malformed JSON beginning with brace must skip")
		})
	}
}

func TestParseWebhookEntry_EmptyValue_Skips(t *testing.T) {
	cases := []string{"", "   ", "\t", "\n"}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			_, skip := k8s.ParseWebhookEntryForTest("name", raw)
			assert.True(t, skip, "empty/whitespace value must skip")
		})
	}
}

func TestParseWebhookEntry_InvalidTimeout_FallsBackToDefault(t *testing.T) {
	// Operator typos in timeout (e.g., "5xyz" instead of "5s") MUST NOT
	// reject the whole entry — fall back to default 30s and dispatch.
	entry, skip := k8s.ParseWebhookEntryForTest("hook",
		`{"url":"https://example.com/x","timeout":"not-a-duration"}`)
	require.False(t, skip, "invalid timeout must fall back to default, not skip")
	assert.Equal(t, "https://example.com/x", entry.URL)
	assert.Equal(t, k8s.DefaultWebhookTimeoutForTest(), entry.Timeout)
}

func TestParseWebhookEntry_NegativeOrZeroTimeout_FallsBackToDefault(t *testing.T) {
	cases := []string{
		`{"url":"https://example.com/x","timeout":"0s"}`,
		`{"url":"https://example.com/x","timeout":"-5s"}`,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			entry, skip := k8s.ParseWebhookEntryForTest("hook", raw)
			require.False(t, skip)
			assert.Equal(t, k8s.DefaultWebhookTimeoutForTest(), entry.Timeout,
				"non-positive timeout must fall back to default")
		})
	}
}

// Plain URL with leading/trailing whitespace is trimmed but NOT
// re-interpreted as JSON.
func TestParseWebhookEntry_PlainURL_Trimmed(t *testing.T) {
	entry, skip := k8s.ParseWebhookEntryForTest("hook", "  https://example.com/x  ")
	require.False(t, skip)
	assert.Equal(t, "https://example.com/x", entry.URL)
}
