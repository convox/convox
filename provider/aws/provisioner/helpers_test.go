package provisioner

import (
	"fmt"
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

func TestGetShortResournce(t *testing.T) {
	r := "cache-rtest-rc3188r-memcache2-elasticache-check"

	fmt.Println(GenShortResourceName(r))
	fmt.Println(r)
}
