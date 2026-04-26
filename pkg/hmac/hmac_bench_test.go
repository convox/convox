package hmac_test

// benchmark targets are advisory; CI does not fail on absolute numbers.
// Hardware variance and Go runtime evolution will drift these targets;
// use go test -bench=. -benchmem locally to spot pathological regressions.

import (
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	cxhmac "github.com/convox/convox/pkg/hmac"
)

// typicalEventBody is the shape and approximate size of a real Convox
// webhook payload. Used as the throughput baseline for sign benchmarks.
var typicalEventBody = []byte(
	`{"action":"app:budget:cap","data":{"app":"demo","cap":"100","rack":"smoke-1021","spend":"12.34","actor":"system","reason":"budget exceeded"},"status":"success","timestamp":"2026-04-24T12:00:00Z"}`,
)

func benchKey(b *testing.B) []byte {
	b.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		b.Fatal(err)
	}
	return buf
}

// BenchmarkSign_TypicalPayload — target <10us/op (advisory).
func BenchmarkSign_TypicalPayload(b *testing.B) {
	key := benchKey(b)
	t := time.Now().Unix()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cxhmac.Sign(t, typicalEventBody, key)
	}
}

// BenchmarkSignedHeader_OneKey — target <15us (advisory).
func BenchmarkSignedHeader_OneKey(b *testing.B) {
	keys := [][]byte{benchKey(b)}
	t := time.Now().Unix()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cxhmac.SignedHeader(t, typicalEventBody, keys)
	}
}

// BenchmarkSignedHeader_TwoKeys — target <25us (advisory).
func BenchmarkSignedHeader_TwoKeys(b *testing.B) {
	keys := [][]byte{benchKey(b), benchKey(b)}
	t := time.Now().Unix()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cxhmac.SignedHeader(t, typicalEventBody, keys)
	}
}

// BenchmarkVerify_OneKey — target <20us (advisory).
func BenchmarkVerify_OneKey(b *testing.B) {
	key := benchKey(b)
	t := time.Now().Unix()
	header := cxhmac.SignedHeader(t, typicalEventBody, [][]byte{key})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cxhmac.Verify(typicalEventBody, header, [][]byte{key}, 5*time.Minute)
	}
}

// BenchmarkSign_1000Webhooks_Throughput — sequential sign linearity.
// No per-op target; b.N captures the full throughput curve.
func BenchmarkSign_1000Webhooks_Throughput(b *testing.B) {
	key := benchKey(b)
	t := time.Now().Unix()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			_ = cxhmac.Sign(t, typicalEventBody, key)
		}
	}
}

// (intentionally exported helper for the linter test)
var _ = hex.EncodeToString
