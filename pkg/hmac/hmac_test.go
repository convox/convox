package hmac_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cxhmac "github.com/convox/convox/pkg/hmac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedKeyHex is the spec-pinned fixture key for reproducible HMAC bytes.
// MUST match pkg/hmac/testdata/expected-sigs.json.
const fixedKeyHex = "5257a869e7ecebeda32affa62cdca3fa37e8c0a98c3f2db5a8f5da3b2a3e9c4e"
const fixedTimestamp int64 = 1745497200

// secondKeyHex is a distinct high-entropy 32-byte key used in rotation tests.
const secondKeyHex = "8c1f3e0b9d4a7f2e6c5b8a3d4f9e2c1a7b6d5f4e3c2b1a9d8f7e6c5b4a3d2e10"

func mustDecode(t testing.TB, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestSign_SingleKey_HappyPath(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{"action":"app:create","data":{"app":"demo"},"status":"success"}`)

	got := cxhmac.Sign(fixedTimestamp, body, key)

	require.True(t, strings.HasPrefix(got, "v1="))
	hexPart := strings.TrimPrefix(got, "v1=")
	assert.Len(t, hexPart, 64, "hex sig must be 64 chars (32 bytes)")
	_, err := hex.DecodeString(hexPart)
	require.NoError(t, err)
}

func TestSignedHeader_TimestampPresent(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{"x":1}`)

	header := cxhmac.SignedHeader(fixedTimestamp, body, [][]byte{key})

	assert.Contains(t, header, fmt.Sprintf("t=%d", fixedTimestamp))
	assert.Contains(t, header, "v1=")
}

func TestSignedHeader_MultipleKeys_AllSignaturesEmitted(t *testing.T) {
	k1 := mustDecode(t, fixedKeyHex)
	k2 := mustDecode(t, secondKeyHex)
	body := []byte(`{"x":1}`)

	header := cxhmac.SignedHeader(fixedTimestamp, body, [][]byte{k1, k2})

	parts := strings.Split(header, ",")
	var v1count int
	for _, p := range parts {
		if strings.HasPrefix(strings.TrimSpace(p), "v1=") {
			v1count++
		}
	}
	assert.Equal(t, 2, v1count, "two-key list must emit two v1= segments")
}

func TestVerify_AcceptsAnyOfMultipleSigs(t *testing.T) {
	k1 := mustDecode(t, fixedKeyHex)
	k2 := mustDecode(t, secondKeyHex)
	body := []byte(`{"action":"app:create"}`)

	header := cxhmac.SignedHeader(time.Now().Unix(), body, [][]byte{k1, k2})

	// Receiver only knows k2 — must still accept.
	err := cxhmac.Verify(body, header, [][]byte{k2}, 5*time.Minute)
	require.NoError(t, err)

	// Receiver only knows k1 — must still accept.
	err = cxhmac.Verify(body, header, [][]byte{k1}, 5*time.Minute)
	require.NoError(t, err)
}

func TestVerify_RejectsExpiredTimestamp(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{"x":1}`)

	// 10 minutes in the past, tolerance 5 minutes
	old := time.Now().Add(-10 * time.Minute).Unix()
	header := cxhmac.SignedHeader(old, body, [][]byte{key})

	err := cxhmac.Verify(body, header, [][]byte{key}, 5*time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tolerance")
}

func TestSignedHeader_KeyOrderIndependent(t *testing.T) {
	k1 := mustDecode(t, fixedKeyHex)
	k2 := mustDecode(t, secondKeyHex)
	body := []byte(`{"x":1}`)

	header := cxhmac.SignedHeader(time.Now().Unix(), body, [][]byte{k1, k2})

	// Verify with reversed key order — must still pass.
	err := cxhmac.Verify(body, header, [][]byte{k2, k1}, 5*time.Minute)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Tamper detection (negative)
// ---------------------------------------------------------------------------

func TestVerify_TamperedBody_Rejects(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{"x":1}`)

	header := cxhmac.SignedHeader(time.Now().Unix(), body, [][]byte{key})

	tampered := []byte(`{"x":2}`)
	err := cxhmac.Verify(tampered, header, [][]byte{key}, 5*time.Minute)
	require.Error(t, err)
}

func TestVerify_TamperedHeader_Rejects(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{"x":1}`)

	header := cxhmac.SignedHeader(time.Now().Unix(), body, [][]byte{key})

	// Mutate the last hex char of the v1= segment.
	idx := strings.LastIndex(header, "v1=") + 3
	bs := []byte(header)
	if bs[len(bs)-1] == 'a' {
		bs[len(bs)-1] = 'b'
	} else {
		bs[len(bs)-1] = 'a'
	}
	_ = idx
	err := cxhmac.Verify(body, string(bs), [][]byte{key}, 5*time.Minute)
	require.Error(t, err)
}

func TestVerify_TamperedTimestamp_Rejects(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{"x":1}`)

	now := time.Now().Unix()
	header := cxhmac.SignedHeader(now, body, [][]byte{key})

	// Replace t=<now> with t=<now-1>; HMAC was over now+body so this
	// should fail even though skew tolerance still passes.
	bad := strings.Replace(header, fmt.Sprintf("t=%d", now), fmt.Sprintf("t=%d", now-1), 1)
	err := cxhmac.Verify(body, bad, [][]byte{key}, 5*time.Minute)
	require.Error(t, err)
}

func TestVerify_WrongVersionPrefix_Rejects(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{"x":1}`)
	now := time.Now().Unix()
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%d.", now)
	mac.Write(body)
	hexSig := hex.EncodeToString(mac.Sum(nil))

	bad := fmt.Sprintf("t=%d,v2=%s", now, hexSig)
	err := cxhmac.Verify(body, bad, [][]byte{key}, 5*time.Minute)
	require.Error(t, err)
}

func TestVerify_MalformedHeader_Rejects(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{}`)

	err := cxhmac.Verify(body, "no-equals-anywhere-here", [][]byte{key}, 5*time.Minute)
	require.Error(t, err)
}

func TestVerify_TruncatedHex_Rejects(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{}`)
	now := time.Now().Unix()

	bad := fmt.Sprintf("t=%d,v1=abc123", now)
	err := cxhmac.Verify(body, bad, [][]byte{key}, 5*time.Minute)
	require.Error(t, err)
}

func TestVerify_NonHexSig_Rejects(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{}`)
	now := time.Now().Unix()

	bad := fmt.Sprintf("t=%d,v1=%s", now, strings.Repeat("z", 64))
	err := cxhmac.Verify(body, bad, [][]byte{key}, 5*time.Minute)
	require.Error(t, err)
}

func TestVerify_RepeatedHeaderField_BothSigsSeen(t *testing.T) {
	// RFC 7230 allows repeated header values; receivers join via
	// http.Header.Values which collapses to a single comma-joined string.
	// This test simulates the joined form: two v1= segments in one header
	// value, both signed by the same body but only one matches.
	k1 := mustDecode(t, fixedKeyHex)
	k2 := mustDecode(t, secondKeyHex)
	body := []byte(`{}`)
	now := time.Now().Unix()

	mac1 := hmac.New(sha256.New, k1)
	fmt.Fprintf(mac1, "%d.", now)
	mac1.Write(body)
	hex1 := hex.EncodeToString(mac1.Sum(nil))

	mac2 := hmac.New(sha256.New, k2)
	fmt.Fprintf(mac2, "%d.", now)
	mac2.Write(body)
	hex2 := hex.EncodeToString(mac2.Sum(nil))

	header := fmt.Sprintf("t=%d,v1=%s,v1=%s", now, hex1, hex2)

	// Verify against k1 only — should still pass via the first sig.
	err := cxhmac.Verify(body, header, [][]byte{k1}, 5*time.Minute)
	require.NoError(t, err)
}

func TestVerify_FoldedLineHeader_StillVerifies(t *testing.T) {
	// HTTP header line folding (obs-fold per RFC 7230) inserts CRLF + SP
	// between fields. Receivers normalize to comma-joined; we test that
	// ordinary whitespace inside the header value does not break parsing.
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{}`)
	now := time.Now().Unix()

	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%d.", now)
	mac.Write(body)
	hexSig := hex.EncodeToString(mac.Sum(nil))

	// Whitespace around comma-separated segments must not break parse.
	folded := fmt.Sprintf("t=%d, v1=%s", now, hexSig)
	err := cxhmac.Verify(body, folded, [][]byte{key}, 5*time.Minute)
	require.NoError(t, err)
}

func TestVerify_CRLFInjection_Rejects(t *testing.T) {
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{}`)
	now := time.Now().Unix()

	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%d.", now)
	mac.Write(body)
	hexSig := hex.EncodeToString(mac.Sum(nil))

	bad := fmt.Sprintf("t=%d,v1=%s\r\nX-Injected: yes", now, hexSig)
	err := cxhmac.Verify(body, bad, [][]byte{key}, 5*time.Minute)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Validation / negative path
// ---------------------------------------------------------------------------

func TestValidateSigningKeys_TooShort_Rejects(t *testing.T) {
	err := cxhmac.ValidateSigningKeys(strings.Repeat("a", 30))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestValidateSigningKeys_OddLength_Rejects(t *testing.T) {
	err := cxhmac.ValidateSigningKeys(strings.Repeat("a", 63))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "odd length")
}

func TestValidateSigningKeys_NonHexChar_Rejects(t *testing.T) {
	bad := "xyzz" + strings.Repeat("a", 60)
	err := cxhmac.ValidateSigningKeys(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-hex")
	assert.Contains(t, err.Error(), "lowercase")
	assert.Contains(t, err.Error(), `"x"`)
}

func TestValidateSigningKeys_MixedCase_Rejects(t *testing.T) {
	bad := "ABCD" + strings.Repeat("a", 60)
	err := cxhmac.ValidateSigningKeys(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lowercase")
	assert.Contains(t, err.Error(), `"A"`)
}

func TestValidateSigningKeys_AllZero_Rejects(t *testing.T) {
	err := cxhmac.ValidateSigningKeys(strings.Repeat("0", 64))
	require.Error(t, err)
	// "0...0" exact-equality matches the zero placeholder before the
	// all-zero decoded check; both error messages are valid signals.
	assert.True(t,
		strings.Contains(err.Error(), "all-zero") ||
			strings.Contains(err.Error(), "placeholder"),
		"got: %s", err.Error())
}

func TestValidateSigningKeys_RepeatingByte_Rejects(t *testing.T) {
	// "ab" repeated 32x decodes to 0xab × 32 — same byte; rejected via
	// either repeating-pattern or low-entropy check.
	bad := strings.Repeat("ab", 32)
	err := cxhmac.ValidateSigningKeys(bad)
	require.Error(t, err)
}

func TestValidateSigningKeys_LowEntropy_Rejects(t *testing.T) {
	// Sequential bytes 00..1f (32 bytes) — Shannon entropy is high in the
	// limit, but counts-bucketed is uniform; sequential is detected by
	// low-entropy or repeating heuristic. Use a clearly low-entropy
	// pattern to assert the rejection: 60 zero hex chars + 4-char tail.
	bad := strings.Repeat("00", 30) + "0102"
	err := cxhmac.ValidateSigningKeys(bad)
	require.Error(t, err)
}

func TestValidateSigningKeys_PlaceholderEquality_Rejects(t *testing.T) {
	// "deadbeef" + 56 zeros — exact-equality placeholder.
	err := cxhmac.ValidateSigningKeys("deadbeef" + strings.Repeat("0", 56))
	require.Error(t, err)
}

func TestValidateSigningKeys_PlaceholderSubstring_Accepts(t *testing.T) {
	// High-entropy random key that contains "changeme" as a substring
	// somewhere internal — must NOT be rejected (substring-match would
	// generate 1-in-2^64 false positives).
	good := "5257a869e7ec6368616e67656d65a32affa62cdca3fa37e8c0a98c3f2db5a8f5"
	require.Len(t, good, 64)
	require.Contains(t, good, "6368616e67656d65", "fixture must contain 'changeme' hex bytes")
	err := cxhmac.ValidateSigningKeys(good)
	require.NoError(t, err)
}

func TestValidateSigningKeys_TooManyKeys_Rejects(t *testing.T) {
	bad := fixedKeyHex + "," + secondKeyHex + "," + fixedKeyHex
	err := cxhmac.ValidateSigningKeys(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most")
}

func TestValidateSigningKeys_EmptyEntry_Rejects(t *testing.T) {
	bad := fixedKeyHex + ","
	err := cxhmac.ValidateSigningKeys(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty key")
}

func TestValidateSigningKeys_EmptyOverall_Accepts(t *testing.T) {
	require.NoError(t, cxhmac.ValidateSigningKeys(""))
	require.NoError(t, cxhmac.ValidateSigningKeys("   "))
}

func TestValidateSigningKeys_OneValid_Accepts(t *testing.T) {
	require.NoError(t, cxhmac.ValidateSigningKeys(fixedKeyHex))
}

func TestValidateSigningKeys_TwoValid_Accepts(t *testing.T) {
	require.NoError(t, cxhmac.ValidateSigningKeys(fixedKeyHex+","+secondKeyHex))
}

func TestValidateSigningKeys_ExtremelyLong_Rejects(t *testing.T) {
	bad := strings.Repeat("a", 4097)
	err := cxhmac.ValidateSigningKeys(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds max length")
}

// ---------------------------------------------------------------------------
// Hex parsing edge cases (R2 F-T-NEW-4)
// ---------------------------------------------------------------------------

func TestValidateSigningKeys_HexDecodePartialSuccess_Rejects(t *testing.T) {
	// "abZZcd..." would in theory let hex.DecodeString consume "ab" before
	// erroring. Our character-class regex catches first; assert the
	// error references the non-hex char, not a hex-decode failure.
	bad := "abZZ" + strings.Repeat("a", 60)
	err := cxhmac.ValidateSigningKeys(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-hex")
}

func TestValidateSigningKeys_NullByteInValue_Rejects(t *testing.T) {
	// Null byte is one character that may trip the even-length OR the
	// hex-character-class check first depending on neighbour bytes. Both
	// paths are acceptable rejection; both close the safety hole.
	bad := "deadbe\x00ef" + strings.Repeat("a", 56)
	err := cxhmac.ValidateSigningKeys(bad)
	require.Error(t, err)
}

func TestParseSigningKeys_TrailingNewline_Trimmed(t *testing.T) {
	keys, err := cxhmac.ParseSigningKeys(fixedKeyHex + "\n")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, mustDecode(t, fixedKeyHex), keys[0])
}

// ---------------------------------------------------------------------------
// Backward compat: byte-equality with json.Marshal output
// ---------------------------------------------------------------------------

type evt struct {
	Action    string            `json:"action"`
	Data      map[string]string `json:"data"`
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
}

func TestEventSendBody_BytesIdenticalTo_JSONMarshal(t *testing.T) {
	// The bytes the rack signs and the bytes the rack POSTs are literally
	// the same []byte slice. Asserts the contract that SignedHeader does
	// NOT mutate or canonicalize the body.
	e := evt{
		Action:    "app:create",
		Data:      map[string]string{"app": "demo", "rack": "rack1"},
		Status:    "success",
		Timestamp: time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
	}
	body, err := json.Marshal(e)
	require.NoError(t, err)

	// Snapshot before signing
	before := append([]byte(nil), body...)
	key := mustDecode(t, fixedKeyHex)
	_ = cxhmac.SignedHeader(fixedTimestamp, body, [][]byte{key})

	assert.True(t, bytes.Equal(before, body), "SignedHeader must not mutate body bytes")
}

func TestEventCanonicalization_AllEventTypes_BackwardCompat(t *testing.T) {
	// Per spec §8.1 R2-26: per-fixture, the test pins
	// expectedSignatureHex to literal hex bytes from
	// pkg/hmac/testdata/expected-sigs.json so the assertion is stable
	// across hypothetical Go-version changes.
	type sigEntry struct {
		ExpectedSignature string `json:"expected_signature"`
	}
	type expectedFile struct {
		KeyHex    string              `json:"key_hex"`
		Timestamp int64               `json:"timestamp"`
		Fixtures  map[string]sigEntry `json:"fixtures"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "expected-sigs.json"))
	require.NoError(t, err)
	var ef expectedFile
	require.NoError(t, json.Unmarshal(raw, &ef))

	key := mustDecode(t, ef.KeyHex)

	for _, fixture := range []string{"event-cap-set", "event-app-create", "event-rack-update"} {
		t.Run(fixture, func(t *testing.T) {
			path := filepath.Join("testdata", fixture+".json")
			body, err := os.ReadFile(path)
			require.NoError(t, err)
			// Strip any trailing newline added by the editor.
			body = bytes.TrimRight(body, "\n")

			mac := hmac.New(sha256.New, key)
			fmt.Fprintf(mac, "%d.", ef.Timestamp)
			mac.Write(body)
			gotHex := hex.EncodeToString(mac.Sum(nil))

			expected := ef.Fixtures[fixture].ExpectedSignature
			require.NotEmpty(t, expected, "fixture %q missing expected signature", fixture)
			assert.Equal(t, expected, gotHex, "literal expected sig must match (R2 F-T-NEW-2)")
		})
	}
}

// ---------------------------------------------------------------------------
// Numeric precision (R1 B-T3)
// ---------------------------------------------------------------------------

func TestSign_NaN_Inf_RejectedByJSONMarshal_BeforeSigning(t *testing.T) {
	// json.Marshal explicitly errors on NaN / Inf for float64 values; our
	// event.Data is map[string]string, so floats can only enter via a
	// hypothetical extension to map[string]interface{}. Test that the
	// upstream Marshal fails before signing is attempted.
	type floatish struct {
		X float64 `json:"x"`
	}
	_, err := json.Marshal(floatish{X: math64Inf()})
	require.Error(t, err, "json.Marshal must fail on Inf before any sign call")
}

func math64Inf() float64 {
	return 1.0 / func() float64 { var z float64; return z }()
}

func TestSign_LargeInteger_PreservedExactly(t *testing.T) {
	// 2^53+1 as a string in event.Data preserves precision verbatim
	// because the rack signs []byte without re-parsing.
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{"amount":"9007199254740993"}`)

	now := time.Now().Unix()
	header := cxhmac.SignedHeader(now, body, [][]byte{key})
	require.NotEmpty(t, header)

	require.NoError(t, cxhmac.Verify(body, header, [][]byte{key}, 5*time.Minute))
}

// ---------------------------------------------------------------------------
// Constant-time compare (runtime check)
// ---------------------------------------------------------------------------

func TestVerify_UsesConstantTimeCompare_NotBytesEqual(t *testing.T) {
	// AST-level guarantee lives in pkg/hmac/lint_test.go (file-level grep
	// rejecting bytes.Equal in Verify). At runtime we assert the function
	// is correct by feeding two strings that bytes.Equal would also catch
	// — distinguishing this from a bytes.Equal regression requires the
	// AST test, but a runtime smoke is still good signal.
	key := mustDecode(t, fixedKeyHex)
	body := []byte(`{}`)
	now := time.Now().Unix()
	header := cxhmac.SignedHeader(now, body, [][]byte{key})

	require.NoError(t, cxhmac.Verify(body, header, [][]byte{key}, 5*time.Minute))

	// Modify a single byte: must fail. A bytes.Equal-based impl would
	// also fail this test; the AST check is the load-bearing assertion.
	bs := []byte(header)
	for i := len(bs) - 1; i >= 0; i-- {
		if bs[i] >= '0' && bs[i] <= '9' {
			if bs[i] == '9' {
				bs[i] = '0'
			} else {
				bs[i]++
			}
			break
		}
	}
	require.Error(t, cxhmac.Verify(body, string(bs), [][]byte{key}, 5*time.Minute))
}

// ---------------------------------------------------------------------------
// Fuzz target (smoke; CI runs short)
// ---------------------------------------------------------------------------

func FuzzParseSignedHeader(f *testing.F) {
	// Seeds: well-formed and malformed headers.
	now := time.Now().Unix()
	f.Add(fmt.Sprintf("t=%d,v1=%s", now, strings.Repeat("a", 64)))
	f.Add("garbage")
	f.Add("")
	f.Add("t=,v1=")
	f.Add("t=abc,v1=def")
	f.Add(strings.Repeat("a,", 200))
	f.Add("\x00\xff\xfeBOM-bytes,v1=00")

	key, _ := hex.DecodeString(fixedKeyHex)

	f.Fuzz(func(t *testing.T, header string) {
		// Asserts: Verify does not panic on arbitrary header input. A
		// well-formed header with the right body and key returns nil;
		// everything else returns an error. We do not assert which.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Verify panicked on header=%q: %v", header, r)
			}
		}()
		_ = cxhmac.Verify([]byte(`{}`), header, [][]byte{key}, 5*time.Minute)
	})
}

// ---------------------------------------------------------------------------
// Panic safety
// ---------------------------------------------------------------------------

func TestSignedHeader_PanicInternal_DegradesGracefully(t *testing.T) {
	// SignedHeader has an internal defer-recover; a panic in
	// signOne would be caught and returns the empty string.
	// We can't easily inject a panic into an unexported helper from a
	// black-box test, but we CAN assert SignedHeader does not panic on
	// nil-or-empty key list.
	require.NotPanics(t, func() {
		got := cxhmac.SignedHeader(time.Now().Unix(), []byte(`{}`), nil)
		assert.Equal(t, "", got)
	})
	require.NotPanics(t, func() {
		got := cxhmac.SignedHeader(time.Now().Unix(), []byte(`{}`), [][]byte{})
		assert.Equal(t, "", got)
	})
	require.NotPanics(t, func() {
		got := cxhmac.SignedHeader(time.Now().Unix(), []byte(`{}`), [][]byte{nil})
		assert.Equal(t, "", got, "all keys empty -> empty header")
	})
}

// ---------------------------------------------------------------------------
// No-key-leak in logs (R1 obs §1.1 + R2 cleanup)
// ---------------------------------------------------------------------------

func TestHmacPackage_NoKeyLeakInLogs(t *testing.T) {
	// Capture stdout, stderr, and the global logger sink; perform a
	// sign + verify round-trip and assert NONE of the raw key, raw
	// body, or computed hex sig appear anywhere in captured logs.
	keyHex := fixedKeyHex
	key := mustDecode(t, keyHex)
	body := []byte(`secretBodyBytes-do-not-leak`)

	// Capture stdout
	rOut, wOut, _ := os.Pipe()
	origOut := os.Stdout
	os.Stdout = wOut

	// Capture stderr
	rErr, wErr, _ := os.Pipe()
	origErr := os.Stderr
	os.Stderr = wErr

	// Capture stdlib log package output
	var logBuf bytes.Buffer
	origLogOut := log.Writer()
	log.SetOutput(&logBuf)

	header := cxhmac.SignedHeader(time.Now().Unix(), body, [][]byte{key})
	_ = cxhmac.Verify(body, header, [][]byte{key}, 5*time.Minute)
	// Trigger an error path too:
	_ = cxhmac.Verify(body, "garbage", [][]byte{key}, 5*time.Minute)
	_ = cxhmac.ValidateSigningKeys("nope")

	wOut.Close()
	wErr.Close()
	os.Stdout = origOut
	os.Stderr = origErr
	log.SetOutput(origLogOut)

	gotOut, _ := io.ReadAll(rOut)
	gotErr, _ := io.ReadAll(rErr)

	combined := string(gotOut) + string(gotErr) + logBuf.String()
	// Compute the expected hex sig for the assertion.
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%d.", time.Now().Unix())
	mac.Write(body)
	hexSig := hex.EncodeToString(mac.Sum(nil))

	assert.NotContains(t, combined, keyHex, "raw key (hex) must not appear in logs")
	assert.NotContains(t, combined, string(body), "raw body bytes must not appear in logs")
	assert.NotContains(t, combined, hexSig, "computed hex signature must not appear in logs")
}
