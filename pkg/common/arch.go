package common

import (
	"sort"
	"strings"
)

// BuildArchs parses a comma-separated architecture list into a sorted, deduplicated set containing only amd64 and arm64.
func BuildArchs(s string) []string {
	seen := map[string]bool{}
	archs := []string{}

	for _, a := range strings.Split(s, ",") {
		a = strings.TrimSpace(a)
		if (a == "amd64" || a == "arm64") && !seen[a] {
			seen[a] = true
			archs = append(archs, a)
		}
	}

	sort.Strings(archs)

	return archs
}
