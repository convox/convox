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

// maxHexLen guards against file-paste DoS.
const maxHexLen = 4096

// maxKeys caps the rotation list length (4 = comfortable rotation depth).
const maxKeys = 4

// MaxSigningKeys exports maxKeys for cross-package validation.
const MaxSigningKeys = maxKeys

// hexCharClass matches lowercase hex only; uppercase rejected for clear error messages.
var hexCharClass = regexp.MustCompile(`^[0-9a-f]+$`)

// nonHexLowerClass finds the first non-hex character for error reporting.
var nonHexLowerClass = regexp.MustCompile(`[^0-9a-f]`)

// placeholderHexValues are known-bad hex strings checked by exact equality.
var placeholderHexValues = []string{
	strings.Repeat("0", 64),
	strings.Repeat("1", 64),
	"deadbeef" + strings.Repeat("0", 56),
	// "changeme" + 56 zero hex chars
	"6368616e67656d65" + strings.Repeat("0", 48),
	// All-F bytes (64 lowercase f characters)
	strings.Repeat("f", 64),
	// "testkey" zero-padded to 32 bytes
	"746573746b657900000000000000000000000000000000000000000000000000",
	// "password" zero-padded to 32 bytes
	"70617373776f7264000000000000000000000000000000000000000000000000",
	// Sequential hex digit pattern repeated 4x (trivially guessable)
	"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
}

// abs returns |x| (Go has no int64 abs).
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// shannonEntropyBitsPerByte computes Shannon entropy of key bytes.
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

func signOne(t int64, body []byte, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%d.", t)
	mac.Write(body)
	return mac.Sum(nil)
}

// Sign returns the v1=<hex> segment for a single key.
func Sign(t int64, body []byte, key []byte) string {
	return "v1=" + hex.EncodeToString(signOne(t, body, key))
}

// SignedHeader returns the full Convox-Signature header value for one or
// more keys. Panics are recovered to degrade to unsigned dispatch.
func SignedHeader(t int64, body []byte, keys [][]byte) (header string) {
	defer func() {
		if r := recover(); r != nil {
			// Do not log: recovered value may carry key material.
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

// Verify checks body against at least one key within the tolerance window.
func Verify(body []byte, header string, keys [][]byte, tolerance time.Duration) error {
	if header == "" {
		return errors.New("missing Convox-Signature header")
	}

	// CRLF injection guard.
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
	// Sub-second tolerance truncates to 0; callers must use whole seconds.
	if abs(now-t) > int64(tolerance.Seconds()) {
		return errors.New("timestamp outside tolerance window")
	}

	sigs := fields["v1"]
	if len(sigs) == 0 {
		return errors.New("no v1 signatures in header")
	}

	for _, sigHex := range sigs {
		// Hex-shape guard before constant-time compare.
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

// ParseSigningKeys parses a comma-separated webhook_signing_key rack param
// into decoded key bytes. Empty input returns (nil, nil).
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
		// Defensive: validated above but avoid zero-length key in signing path.
		if s == "" {
			continue
		}
		b, err := hex.DecodeString(s)
		if err != nil {
			// Unreachable post-validation.
			return nil, fmt.Errorf("hex decode failure post-validation: %w", err)
		}
		out = append(out, b)
	}
	return out, nil
}

// ValidateSigningKeys enforces hex format, minimum length, weak-key
// rejection, and max-key-count. Error messages avoid echoing key contents.
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
		for _, ph := range placeholderHexValues {
			if s == ph {
				return fmt.Errorf("webhook_signing_key: key #%d matches a known placeholder value; use 'openssl rand -hex 32' to generate a real key", idx)
			}
		}
		decoded, err := hex.DecodeString(s)
		if err != nil {
			// Unreachable post-validation.
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
