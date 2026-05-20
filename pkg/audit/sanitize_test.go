package audit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSanitizeActor_DefenseInDepthStrips locks in the universal-actor
// sanitizer. Each case pins a specific Unicode-class strip rule. A
// regression of any individual rule (e.g. a future refactor that
// drops the C1 range check) would break the matching case here.
//
// Migrated from provider/k8s/budget_cost_test.go::
// TestSanitizeAckBy_DefenseInDepthStrips when the sanitizer was
// promoted out of provider/k8s into the shared pkg/audit package so
// the rack auth middleware (pkg/api) and budget accumulator
// (provider/k8s) share a single canonical implementation. The
// budget-side test in budget_cost_test.go is preserved as a
// regression guard against accidental sanitizer drift in the move.
//
// Non-ASCII inputs use Go's backslash-u escape syntax so source-
// embedded invisible characters cannot confuse the parser or hide
// intent.
func TestSanitizeActor_DefenseInDepthStrips(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// C0 + DEL.
		{"C0_NUL", "alice\x00bob", "alicebob"},
		{"C0_LF", "alice\nbob", "alicebob"},
		{"C0_CR", "alice\rbob", "alicebob"},
		{"C0_TAB", "alice\tbob", "alicebob"},
		{"DEL_0x7F", "alice\x7fbob", "alicebob"},

		// C1 controls (0x80-0x9f) — legacy terminal escape sequences.
		{"C1_low_0x80", "alice\u0080bob", "alicebob"},
		{"C1_CSI_0x9b", "alice\u009bbob", "alicebob"},
		{"C1_high_0x9f", "alice\u009fbob", "alicebob"},

		// BiDi overrides — display-spoofing (rendered text reverses).
		{"BiDi_LRE_U202A", "alice\u202abob", "alicebob"},
		{"BiDi_RLE_U202B", "alice\u202bbob", "alicebob"},
		{"BiDi_PDF_U202C", "alice\u202cbob", "alicebob"},
		{"BiDi_LRO_U202D", "alice\u202dbob", "alicebob"},
		{"BiDi_RLO_U202E", "alice\u202ebob", "alicebob"},
		{"BiDi_LRI_U2066", "alice\u2066bob", "alicebob"},
		{"BiDi_RLI_U2067", "alice\u2067bob", "alicebob"},
		{"BiDi_FSI_U2068", "alice\u2068bob", "alicebob"},
		{"BiDi_PDI_U2069", "alice\u2069bob", "alicebob"},

		// Line/paragraph separators — legacy JSON parser break-out and
		// renderer-line-break injection.
		{"LSEP_U2028", "alice\u2028bob", "alicebob"},
		{"PSEP_U2029", "alice\u2029bob", "alicebob"},

		// Zero-width characters — invisible-character spoofing of
		// audit-log values.
		{"ZWSP_U200B", "alice\u200bbob", "alicebob"},
		{"ZWNJ_U200C", "alice\u200cbob", "alicebob"},
		{"ZWJ_U200D", "alice\u200dbob", "alicebob"},
		{"LRM_U200E", "alice\u200ebob", "alicebob"},
		{"RLM_U200F", "alice\u200fbob", "alicebob"},

		// Byte order mark — invisible-character spoofing.
		{"BOM_UFEFF", "alice\ufeffbob", "alicebob"},

		// Truthful values pass through unmodified.
		{"truthful_email", "alice@example.com", "alice@example.com"},

		// Whitespace-only collapses to "unknown" so a whitespace-only
		// input cannot stamp a misleading actor on the event.
		{"whitespace_spaces", "   ", "unknown"},
		{"whitespace_tabs", "\t\t", "unknown"},
		{"whitespace_mixed", " \t \n ", "unknown"},
		{"whitespace_unicode_NBSP", "\u00a0\u00a0", "unknown"},
		{"whitespace_unicode_NNBSP", "\u202f", "unknown"},

		// Empty input also falls through to "unknown".
		{"empty", "", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SanitizeActor(tc.in),
				"SanitizeActor(%q) must equal %q", tc.in, tc.want)
		})
	}
}

// TestSanitizeActor_LengthCap pins the 256-rune cap. Annotations on
// k8s namespaces are bounded; an unbounded ack_by would let a hostile
// caller exhaust the annotation budget in one write. The cap matches
// the budget_accumulator existing convention.
func TestSanitizeActor_LengthCap(t *testing.T) {
	long := strings.Repeat("a", 1024)
	assert.Equal(t, 256, len(SanitizeActor(long)),
		"SanitizeActor must cap output at 256 runes")
}

// TestSanitizeActor_HostileBidiZWInOneString pins the canonical
// header-bridge attack vector: a single hostile header containing a
// right-to-left override (U+202E) plus a zero-width space (U+200B)
// must collapse to a rendered-truthful string with no display-
// spoofing or invisible chars. Defense-in-depth requirement per the
// architecture review for the actor-attribution header bridge
// (pkg/api auth middleware will pass header-supplied actor strings
// through this helper before stamping ConvoxJwtUserParam).
func TestSanitizeActor_HostileBidiZWInOneString(t *testing.T) {
	in := "alice@example.com\u202etest\u200bfoo"
	got := SanitizeActor(in)
	assert.Equal(t, "alice@example.comtestfoo", got,
		"hostile bidi/zw chars must be stripped in one pass")
}
