package rds_test

import (
	"testing"

	"github.com/convox/convox/provider/aws/provisioner/rds"
	"github.com/stretchr/testify/assert"
)

func TestEnsureParameterMetaDate(t *testing.T) {
	for _, p := range rds.ParametersNameList() {
		_, err := rds.ParameterMetaDataForInstall(p)
		assert.NoError(t, err)
	}
}
