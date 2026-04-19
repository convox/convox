package cli

import (
	"sort"
	"strings"
	"testing"
)

func TestResolveGroupExactMatch(t *testing.T) {
	got, err := resolveGroup("karpenter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "karpenter" {
		t.Errorf("expected 'karpenter', got %q", got)
	}
}

func TestResolveGroupPrefixUnique(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"karp", "karpenter"},
		{"k", "karpenter"},
		{"net", "network"},
		{"nod", "nodes"},
		{"sec", "security"},
		{"sca", "scaling"},
		{"sto", "storage"},
		{"reg", "registry"},
		{"ret", "retention"},
		{"b", "build"},
		{"l", "logging"},
		{"i", "ingress"},
		{"d", "domain"},
		{"v", "versions"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := resolveGroup(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("resolveGroup(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestResolveGroupCaseInsensitive(t *testing.T) {
	got, err := resolveGroup("KARPENTER")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "karpenter" {
		t.Errorf("expected 'karpenter', got %q", got)
	}
}

func TestResolveGroupTrimsWhitespace(t *testing.T) {
	got, err := resolveGroup("  karp  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "karpenter" {
		t.Errorf("expected 'karpenter', got %q", got)
	}
}

func TestResolveGroupAmbiguousPrefix(t *testing.T) {
	cases := []struct {
		input       string
		wantMatches []string
		wantHint    string
	}{
		{"n", []string{"network", "nodes"}, "(use 'net' or 'nod')"},
		{"s", []string{"scaling", "security", "storage"}, "(use 'sca', 'sec', or 'sto')"},
		{"r", []string{"registry", "retention"}, "(use 'reg' or 'ret')"},
		{"re", []string{"registry", "retention"}, "(use 'reg' or 'ret')"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			_, err := resolveGroup(tc.input)
			if err == nil {
				t.Fatalf("expected error for ambiguous input %q, got nil", tc.input)
			}
			msg := err.Error()
			if !strings.Contains(msg, "matches multiple groups") {
				t.Errorf("error should mention 'matches multiple groups', got: %s", msg)
			}
			for _, m := range tc.wantMatches {
				if !strings.Contains(msg, m) {
					t.Errorf("error should name candidate %q, got: %s", m, msg)
				}
			}
			if !strings.Contains(msg, tc.wantHint) {
				t.Errorf("error should contain disambiguation hint %q, got: %s", tc.wantHint, msg)
			}
		})
	}
}

func TestResolveGroupNotFound(t *testing.T) {
	_, err := resolveGroup("nope")
	if err == nil {
		t.Fatal("expected error for unknown group, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "available groups") {
		t.Errorf("error should list available groups, got: %s", err.Error())
	}
}

func TestResolveGroupEmptyInput(t *testing.T) {
	for _, input := range []string{"", "   ", "\t\n"} {
		t.Run("input="+input, func(t *testing.T) {
			_, err := resolveGroup(input)
			if err == nil {
				t.Fatal("expected error for empty input, got nil")
			}
			if !strings.Contains(err.Error(), "group name required") {
				t.Errorf("error should mention 'group name required', got: %s", err.Error())
			}
		})
	}
}

func TestResolveGroupCaseInsensitiveAmbiguous(t *testing.T) {
	_, err := resolveGroup("N")
	if err == nil {
		t.Fatal("expected error for ambiguous uppercase input, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "matches multiple groups") {
		t.Errorf("error should mention 'matches multiple groups', got: %s", msg)
	}
	if !strings.Contains(msg, "network") || !strings.Contains(msg, "nodes") {
		t.Errorf("error should name candidates network and nodes, got: %s", msg)
	}
}

func TestFormatGroupList(t *testing.T) {
	out := formatGroupList()
	if !strings.HasPrefix(out, "available groups:\n") {
		t.Errorf("output should start with 'available groups:' header, got: %s", out)
	}

	// Compute expected padding width = longest group name length.
	maxLen := 0
	for g := range groupDescriptions {
		if len(g) > maxLen {
			maxLen = len(g)
		}
	}

	// Every group in groupDescriptions must appear as a properly padded row.
	for g, desc := range groupDescriptions {
		expectedRow := "  " + g + strings.Repeat(" ", maxLen-len(g)) + "    " + desc
		if !strings.Contains(out, expectedRow) {
			t.Errorf("missing or mispadded row for group %q; expected to contain %q; full output:\n%s", g, expectedRow, out)
		}
	}

	// Rows must appear in alphabetical order of group name.
	var names []string
	for g := range groupDescriptions {
		names = append(names, g)
	}
	sort.Strings(names)
	lastIdx := -1
	for _, g := range names {
		idx := strings.Index(out, "  "+g+" ")
		if idx <= lastIdx {
			t.Errorf("group %q not in alphabetical position (idx=%d, prev=%d)", g, idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestFormatAmbiguousHint(t *testing.T) {
	cases := []struct {
		name       string
		candidates []string
		want       string
	}{
		{"empty", []string{}, ""},
		{"one", []string{"karpenter"}, "(use 'kar')"},
		{"two", []string{"network", "nodes"}, "(use 'net' or 'nod')"},
		{"three", []string{"scaling", "security", "storage"}, "(use 'sca', 'sec', or 'sto')"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAmbiguousHint(tc.candidates)
			if got != tc.want {
				t.Errorf("formatAmbiguousHint(%v) = %q, want %q", tc.candidates, got, tc.want)
			}
		})
	}
}

func TestDisambiguatingPrefix(t *testing.T) {
	cases := []struct {
		group string
		want  string
	}{
		{"karpenter", "kar"},
		{"build", "bui"},
		{"logging", "log"},
		{"ingress", "ing"},
		{"domain", "dom"},
		{"versions", "ver"},
		{"network", "net"},
		{"nodes", "nod"},
		{"security", "sec"},
		{"scaling", "sca"},
		{"storage", "sto"},
		{"registry", "reg"},
		{"retention", "ret"},
	}
	for _, tc := range cases {
		t.Run(tc.group, func(t *testing.T) {
			got := disambiguatingPrefix(tc.group)
			if got != tc.want {
				t.Errorf("disambiguatingPrefix(%q) = %q, want %q", tc.group, got, tc.want)
			}
		})
	}
}
