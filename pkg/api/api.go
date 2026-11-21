package api

import (
	"crypto/subtle"
	"net/http"
	"reflect"

	"github.com/convox/convox/pkg/audit"
	"github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider"
	"github.com/convox/stdapi"
)

type Server struct {
	*stdapi.Server
	Password string
	Provider structs.Provider
	JwtMngr  *jwt.JwtManager
}

func New() (*Server, error) {
	p, err := provider.FromEnv()
	if err != nil {
		return nil, err
	}

	return NewWithProvider(p), nil
}

func NewWithProvider(p structs.Provider) *Server {
	if err := p.Initialize(structs.ProviderOptions{}); err != nil {
		panic(err)
	}

	if err := p.Start(); err != nil {
		panic(err)
	}

	signKey, err := p.SystemJwtSignKey()
	if err != nil {
		panic(err)
	}

	jwtMngr := jwt.NewJwtManager(signKey)

	s := &Server{
		Provider: p,
		Server:   stdapi.New("api", "api"),
		JwtMngr:  jwtMngr,
	}

	s.Server.Router.Router = s.Server.Router.Router.SkipClean(true)

	s.Subrouter("/", func(auth *stdapi.Router) {
		auth.Use(securityHeaders)
		auth.Use(s.audit)
		auth.Use(s.authenticate)
		auth.Use(s.auditSession)

		auth.Route("GET", "/auth", func(c *stdapi.Context) error { return c.RenderOK() })

		s.setupRoutes(*auth)
	})

	return s
}

func (s *Server) authenticate(next stdapi.HandlerFunc) stdapi.HandlerFunc {
	return func(c *stdapi.Context) error {
		username, pass, _ := c.Request().BasicAuth()
		if username == "jwt" && s.JwtMngr != nil {
			data, err := s.JwtMngr.Verify(pass)
			if err != nil {
				return stdapi.Errorf(http.StatusUnauthorized, "invalid authentication: %s", err)
			}
			c.Set(structs.ConvoxRoleParam, data.Role)
			c.Set(structs.ConvoxJwtUserParam, data.User)
		} else {
			if s.Password != "" && subtle.ConstantTimeCompare([]byte(s.Password), []byte(pass)) != 1 {
				c.Response().Header().Set("WWW-Authenticate", `Basic realm="convox"`)
				return stdapi.Errorf(http.StatusUnauthorized, "invalid authentication")
			}
			SetAdminRole(c)
			c.Set(structs.ConvoxJwtUserParam, audit.Actor(c.Request().Header))
		}

		return next(c)
	}
}

func securityHeaders(next stdapi.HandlerFunc) stdapi.HandlerFunc {
	return func(c *stdapi.Context) error {
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		c.Response().Header().Set("X-Frame-Options", "DENY")
		return next(c)
	}
}

func (s *Server) hook(name string, args ...interface{}) error {
	vfn, ok := reflect.TypeOf(s).MethodByName(name)
	if !ok {
		return nil
	}

	rargs := []reflect.Value{reflect.ValueOf(s)}

	for _, arg := range args {
		rargs = append(rargs, reflect.ValueOf(arg))
	}

	rvs := vfn.Func.Call(rargs)
	if len(rvs) == 0 {
		return nil
	}

	if err, ok := rvs[0].Interface().(error); ok && err != nil {
		return err
	}

	return nil
}

func (s *Server) provider(_ *stdapi.Context) structs.Provider {
	return s.Provider
}
