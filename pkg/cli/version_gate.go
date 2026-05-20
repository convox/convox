package cli

import (
	"fmt"
	"strings"
)

// Matches only "response status 404" — not bare "404" or "not found" —
// to avoid swallowing resource-not-found errors (e.g. misspelled app name).
func isRackVersionGated(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "response status 404")
}

func wrapVersionGate(err error, feature string) error {
	if !isRackVersionGated(err) {
		return err
	}
	return fmt.Errorf("%s requires rack version 3.24.6 or later", feature)
}
