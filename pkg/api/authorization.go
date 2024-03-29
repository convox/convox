package api

import (
	"net/http"
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
)

func (s *Server) Authorize(next stdapi.HandlerFunc) stdapi.HandlerFunc {
	return func(c *stdapi.Context) error {
		switch c.Request().Method {
		case http.MethodGet:
			if !CanRead(c) {
				return stdapi.Errorf(http.StatusUnauthorized, "you are unauthorized to access this")
			}
		default:
			if !CanWrite(c) {
				return stdapi.Errorf(http.StatusUnauthorized, "you are unauthorized to access this")
			}
		}
		return next(c)
	}
}

func CanRead(c *stdapi.Context) bool {
	if d := c.Get(structs.ConvoxRoleParam); d != nil {
		v, _ := d.(string)
		return strings.Contains(v, "r")
	}
	return false
}

func CanWrite(c *stdapi.Context) bool {
	if d := c.Get(structs.ConvoxRoleParam); d != nil {
		v, _ := d.(string)
		return strings.Contains(v, "w")
	}
	return false
}

func SetReadRole(c *stdapi.Context) {
	c.Set(structs.ConvoxRoleParam, structs.ConvoxRoleRead)
}

func SetReadWriteRole(c *stdapi.Context) {
	c.Set(structs.ConvoxRoleParam, structs.ConvoxRoleReadWrite)
}
