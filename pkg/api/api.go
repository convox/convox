package api

import (
	"reflect"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider"
	"github.com/convox/stdapi"
)

type Server struct {
	*stdapi.Server
	Password string
	Provider structs.Provider
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

	s := &Server{
		Provider: p,
		Server:   stdapi.New("api", "api"),
	}

	s.Server.Router.Router = s.Server.Router.Router.SkipClean(true)

	s.Subrouter("/", func(auth *stdapi.Router) {
		auth.Use(s.authenticate)

		auth.Route("GET", "/auth", func(c *stdapi.Context) error { return c.RenderOK() })

		s.setupRoutes(*auth)
	})

	return s
}

func (s *Server) authenticate(next stdapi.HandlerFunc) stdapi.HandlerFunc {
	return func(c *stdapi.Context) error {
		if _, pass, _ := c.Request().BasicAuth(); s.Password != "" && s.Password != pass {
			c.Response().Header().Set("WWW-Authenticate", `Basic realm="convox"`)
			return stdapi.Errorf(401, "invalid authentication")
		}

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
