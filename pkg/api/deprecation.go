package api

import "github.com/convox/convox/pkg/structs"

// deprecationSunsetDate returns the RFC 7231 IMF-fixdate when --ack-by will be
// rejected. Pinned to 6 months from anticipated 3.24.6 cut date; update
// structs.SunsetDate3250 at 3.25.0 release-cut.
func deprecationSunsetDate() string {
	return structs.SunsetDate3250
}
