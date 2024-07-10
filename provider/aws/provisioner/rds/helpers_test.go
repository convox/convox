package rds

import (
	"testing"

	"github.com/convox/convox/pkg/options"
)

func TestConvertToStringPtr(t *testing.T) {
	convertToStringPtr(1)
	convertToStringPtr(options.Int32(123))
	convertToStringPtr(options.Int64(123))
	convertToStringPtr(options.Bool(true))
	convertToStringPtr(options.String("erer"))
	convertToStringPtr([]string{"1"})

	var a *string
	convertToStringPtr(a)

	var v1 *int32
	convertToStringPtr(v1)
}
