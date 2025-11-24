package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceSubstutionId(t *testing.T) {
	rs := &resourceSubstitution{
		App:     "myapp",
		RType:   "mytype",
		RName:   "myname",
		StateId: "mystateid",
		Tid:     "tid",
	}

	id := resourceSubstitutionId(rs)
	assert.Equal(t, "##|app:myapp|rtype:mytype|rname:myname|stateid:mystateid|tid:tid|##", id)

	parsed := parseResourceSubstitutionId(id)
	assert.Equal(t, rs.App, parsed.App)
	assert.Equal(t, rs.RType, parsed.RType)
	assert.Equal(t, rs.RName, parsed.RName)
	assert.Equal(t, rs.StateId, parsed.StateId)
	assert.Equal(t, rs.Tid, parsed.Tid)
}
