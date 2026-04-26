package hmac

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// minHexLen is 32 raw bytes after hex decode (64 hex chars).
const minHexLen = 64

// maxHexLen guards against accidental file-paste DoS into a rack param.
const maxHexLen = 4096

// maxKeys caps the rotation list length per spec §2.5.
const maxKeys = 2

// hexCharClass matches one or more lowercase hex characters. Uppercase is
// REJECTED so the validator can give the customer an actionable "use
// lowercase" hint without ambiguity. See spec §5.1.
var hexCharClass = regexp.MustCompile(`^[0-9a-f]+$`)

// nonHexLowerClass identifies the first offending character in a non-hex
// string so the validator can report its offset and value.
var nonHexLowerClass = regexp.MustCompile(`[^0-9a-f]`)

// placeholderHexValues are exact-equality known-bad hex strings. Substring
// match is intentionally avoided: a high-entropy random key that happens to
// contain "changeme" as a substring is statistically harmless.
var placeholderHexValues = []string{
	strings.Repeat("0", 64),
	strings.Repeat("1", 64),
	"deadbeef" + strings.Repeat("0", 56),
	// "changeme" + 56 zero hex chars
	"6368616e67656d65" + strings.Repeat("0", 48),
}

// abs returns the absolute value of x. Provided here because Go has no
// stdlib int64 abs (math.Abs is float64-only). Spec §12.3.
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// shannonEntropyBitsPerByte computes Shannon entropy of decoded key bytes
// in bits per byte. Sequential / repeating / ASCII-text inputs score low.
func shannonEntropyBitsPerByte(b []byte) float64 {
	if len(b) == 0 {
		return 0
	}
	var counts [256]int
	for _, x := range b {
		counts[x]++
	}
	total := float64(len(b))
	var h float64
	for _, c := range &counts {
		if c == 0 {
			continue
		}
		p := float64(c) / total
		h -= p * math.Log2(p)
	}
	return h
}

// signOne computes HMAC-SHA256(key, "<t>.<body>") and returns hex(mac).
func signOne(t int64, body []byte, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%d.", t)
	mac.Write(body)
	return mac.Sum(nil)
}

// Sign returns the v1=<hex> segment for a single key, given a Unix
// timestamp and the body bytes. The signed input is fmt.Sprintf("%d.%s",
// t, body).
func Sign(t int64, body []byte, key []byte) string {
	return "v1=" + hex.EncodeToString(signOne(t, body, key))
}

// SignedHeader returns the full Convox-Signature header value:
//
//	t=<unix-ts>,v1=<hex1>[,v1=<hex2>]
//
// for one or more keys. Empty keys list returns "". Per spec §12.2 a
// runtime panic in the inner sign call is recovered and logged via the
// returned empty string so callers degrade to unsigned dispatch instead
// of crashing.
func SignedHeader(t int64, body []byte, keys [][]byte) (header string) {
	defer func() {
		if r := recover(); r != nil {
			// Do not log the recovered value: it may carry key or body
			// bytes via internal stack state. The caller observes the
			// empty header and decides whether to log a degraded notice.
			header = ""
		}
	}()

	if len(keys) == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("t=%d", t)}
	for _, k := range keys {
		if len(k) == 0 {
			continue
		}
		parts = append(parts, Sign(t, body, k))
	}
	if len(parts) == 1 {
		return ""
	}
	return strings.Join(parts, ",")
}

// Verify verifies that the Convox-Signature header value authenticates
// the given body bytes against AT LEAST ONE of the provided keys, AND
// that the header timestamp is within the tolerance window. Returns nil
// on success.
//
// Provided in the Go SDK only — Python/Node/Ruby receivers are sampled
// in webhooks.md.
func Verify(body []byte, header string, keys [][]byte, tolerance time.Duration) error {
	if header == "" {
		return errors.New("missing Convox-Signature header")
	}

	// CRLF injection guard: forbid any control byte. Any LF/CR in the
	// header value is anomalous (header line was already parsed before
	// reaching this function) and indicates an injection attempt.
	for i := 0; i < len(header); i++ {
		c := header[i]
		if c == '\r' || c == '\n' || c == '\x00' {
			return errors.New("Convox-Signature contains control bytes")
		}
	}

	fields := map[string][]string{}
	for _, p := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 {
			return errors.New("malformed Convox-Signature segment")
		}
		fields[kv[0]] = append(fields[kv[0]], kv[1])
	}

	tStr, ok := fields["t"]
	if !ok || len(tStr) == 0 {
		return errors.New("missing timestamp")
	}
	t, err := strconv.ParseInt(tStr[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	now := time.Now().Unix()
	// tolerance.Seconds() returns float64; sub-second tolerance truncates
	// to 0 and would reject every webhook. Pin tolerance to whole seconds
	// in callers. (Spec §12.3)
	if abs(now-t) > int64(tolerance.Seconds()) {
		return errors.New("timestamp outside tolerance window")
	}

	sigs := fields["v1"]
	if len(sigs) == 0 {
		return errors.New("no v1 signatures in header")
	}

	for _, sigHex := range sigs {
		// Hex-shape guard before const-time compare; rejects truncated /
		// non-hex / mixed-case sigs without leaking comparison timing.
		if len(sigHex) != 2*sha256.Size {
			continue
		}
		if !hexCharClass.MatchString(sigHex) {
			continue
		}
		gotBytes, err := hex.DecodeString(sigHex)
		if err != nil {
			continue
		}
		for _, k := range keys {
			if len(k) == 0 {
				continue
			}
			expected := signOne(t, body, k)
			// hmac.Equal MUST be the symbol used here (not bytes.Equal).
			// See pkg/hmac/lint_test.go for the AST-level guard.
			if hmac.Equal(gotBytes, expected) {
				return nil
			}
		}
	}
	return errors.New("no signature in Convox-Signature matched any signing key")
}

// ParseSigningKeys parses a comma-separated webhook_signing_key rack-param
// value into a [][]byte of decoded keys. Empty input returns (nil, nil)
// signaling the disabled state. Decoded keys are returned post-hex-decode
// once at boot; subsequent comparisons operate on the raw []byte slices.
// Spec §5.5 boot-time hex-decode-once contract.
func ParseSigningKeys(rackParam string) ([][]byte, error) {
	if err := ValidateSigningKeys(rackParam); err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(rackParam)
	if trimmed == "" {
		return nil, nil
	}
	raw := strings.Split(trimmed, ",")
	out := make([][]byte, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		// ValidateSigningKeys already rejected empty entries; double-check
		// here defensively to avoid a degenerate len(0) []byte in the
		// signing path.
		if s == "" {
			continue
		}
		b, err := hex.DecodeString(s)
		if err != nil {
			// Unreachable post-validation but returned for total-correctness.
			return nil, fmt.Errorf("hex decode failure post-validation: %w", err)
		}
		out = append(out, b)
	}
	return out, nil
}

// ValidateSigningKeys enforces hex format, minimum length (32 bytes after
// decode), weak-key rejection, and max-key-count rules per spec §5.
// Returns nil on success. Error messages are customer-actionable: they
// describe key shape (offset, length, hex-vs-not) without echoing key
// contents to logs.
func ValidateSigningKeys(rackParam string) error {
	trimmed := strings.TrimSpace(rackParam)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	if len(parts) > maxKeys {
		return fmt.Errorf("webhook_signing_key: at most %d keys supported for rotation; got %d", maxKeys, len(parts))
	}

	for i, raw := range parts {
		s := strings.TrimSpace(raw)
		idx := i + 1
		if s == "" {
			return fmt.Errorf("webhook_signing_key: empty key in list (position %d)", idx)
		}
		if len(s) > maxHexLen {
			return fmt.Errorf("webhook_signing_key: key #%d exceeds max length (%d chars; max %d)", idx, len(s), maxHexLen)
		}
		if len(s)%2 != 0 {
			return fmt.Errorf("webhook_signing_key: key #%d has odd length (%d); hex strings must be even-length", idx, len(s))
		}
		if len(s) < minHexLen {
			return fmt.Errorf("webhook_signing_key: key #%d is too short (%d chars = %d bytes); minimum is 32 bytes (64 hex chars). Generate with: openssl rand -hex 32", idx, len(s), len(s)/2)
		}
		if !hexCharClass.MatchString(s) {
			loc := nonHexLowerClass.FindStringIndex(s)
			char := byte('?')
			off := -1
			if loc != nil {
				char = s[loc[0]]
				off = loc[0]
			}
			return fmt.Errorf("webhook_signing_key: key #%d contains non-hex character %q at offset %d; hex validation requires lowercase [0-9a-f]; use 'openssl rand -hex 32' to generate", idx, string(char), off)
		}
		// Placeholder equality (spec §5.2.4)
		for _, ph := range placeholderHexValues {
			if s == ph {
				return fmt.Errorf("webhook_signing_key: key #%d matches a known placeholder value; use 'openssl rand -hex 32' to generate a real key", idx)
			}
		}
		decoded, err := hex.DecodeString(s)
		if err != nil {
			// Unreachable: hexCharClass + even-length checks already passed.
			return fmt.Errorf("webhook_signing_key: key #%d hex decode failed (%v); regenerate with 'openssl rand -hex 32'", idx, err)
		}
		if isAllZero(decoded) {
			return fmt.Errorf("webhook_signing_key: key #%d decodes to all-zero bytes; use cryptographically random bytes from 'openssl rand -hex 32'", idx)
		}
		if isAllSameByte(decoded) {
			return fmt.Errorf("webhook_signing_key: key #%d decodes to a repeating byte pattern; use cryptographically random bytes from 'openssl rand -hex 32'", idx)
		}
		if shannonEntropyBitsPerByte(decoded) < 3.5 {
			return fmt.Errorf("webhook_signing_key: key #%d has low entropy; use 'openssl rand -hex 32' to generate", idx)
		}
	}
	return nil
}

func isAllZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

func isAllSameByte(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	first := b[0]
	for _, x := range b[1:] {
		if x != first {
			return false
		}
	}
	return true
}
