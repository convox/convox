package audit

import "strings"

// SanitizeActor caps length and strips control/bidi/zero-width characters.
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
