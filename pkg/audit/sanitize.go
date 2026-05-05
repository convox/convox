// Package audit provides shared audit-event helpers consumed by the rack's
// API layer (pkg/api auth middleware) and the provider-side event emitters
// (provider/k8s budget accumulator, service-scale-override). Keep the
// surface minimal — sanitization only. Higher-level concerns (event
// emission, payload schema) live in their respective packages so this
// package can be imported by both upstream (pkg/api) and downstream
// (provider/k8s) call sites without circular-import issues.
package audit

import "strings"

// SanitizeActor caps an actor-identity string and strips control characters
// that could DoS audit-log consumers, spoof rendered output, or inject
// terminal-escape / log-injection payloads. The function is the single
// canonical sanitizer used by:
//
//   - pkg/api auth middleware — header-supplied actor strings (Convox-Actor /
//     X-Convox-Actor) before they are stamped into ConvoxJwtUserParam.
//   - provider/k8s budget accumulator — ack_by form-param values before they
//     are persisted on namespace annotations and stamped into audit events.
//   - provider/k8s service-scale-override — ack_by values for the
//     ServiceScaleOverrideSet path.
//
// Behavior:
//   - Caps at 256 runes (matches the budget_accumulator's pre-3.24.6 cap;
//     bounded so a hostile caller cannot exhaust the k8s annotation budget
//     in one write).
//   - Strips C0 controls (< 0x20) and DEL (0x7f) — log-injection.
//   - Strips C1 controls (0x80-0x9f) — legacy terminal escape sequences.
//   - Strips BiDi overrides (U+202A-U+202E, U+2066-U+2069) — display-spoofing
//     attacks (e.g. an RLO embedded in an email address reverses rendered
//     text in audit-log viewers).
//   - Strips line/paragraph separators (U+2028, U+2029) — legacy JSON
//     parser break-out and renderer-line-break injection.
//   - Strips BOM/zero-width characters (U+FEFF, U+200B-U+200F) — invisible-
//     character spoofing of audit-log values (ZWSP, ZWNJ, ZWJ, LRM, RLM).
//   - Whitespace-only input (including unicode NBSP / NNBSP) collapses to
//     "unknown" so a pathological "   " actor cannot stamp a misleading
//     blank value on the event.
//
// Defense in depth: callers should still sanitize their inputs at the
// origin (rack auth middleware on header receive, budget accumulator on
// form-param receive). This helper exists to be the single immovable
// guarantee that NO control char ever reaches the audit-event payload, no
// matter how many call sites add or remove their own pre-sanitization.
func SanitizeActor(in string) string {
	const maxLen = 256
	out := make([]rune, 0, len(in))
	for _, r := range in {
		if r < 0x20 || r == 0x7f {
			continue
		}
		if r >= 0x80 && r <= 0x9f {
			continue
		}
		switch r {
		case 0x2028, 0x2029, 0xfeff,
			0x200b, 0x200c, 0x200d, 0x200e, 0x200f,
			0x202a, 0x202b, 0x202c, 0x202d, 0x202e,
			0x2066, 0x2067, 0x2068, 0x2069:
			continue
		}
		out = append(out, r)
		if len(out) >= maxLen {
			break
		}
	}
	if strings.TrimSpace(string(out)) == "" {
		return "unknown"
	}
	return string(out)
}
