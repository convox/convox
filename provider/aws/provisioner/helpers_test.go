package provisioner

import (
	"testing"

	"github.com/convox/convox/pkg/options"
)

func TestConvertToStringPtr(t *testing.T) {
	ConvertToStringPtr(1)
	ConvertToStringPtr(options.Int32(123))
	ConvertToStringPtr(options.Int64(123))
	ConvertToStringPtr(options.Bool(true))
	ConvertToStringPtr(options.String("erer"))
	ConvertToStringPtr([]string{"1"})

	var a *string
	ConvertToStringPtr(a)

	var v1 *int32
	ConvertToStringPtr(v1)
}
