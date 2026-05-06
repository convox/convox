package api

import (
	"net/http"
	"reflect"
	"strings"

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

	// s.Router.HandleFunc("/debug/pprof/", pprof.Index)
	// s.Router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	// s.Router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	// s.Router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	// s.Router.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// s.Route("GET", "/v2/", func(c *stdapi.Context) error {
	// 	c.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
	// 	if _, pass, _ := c.Request().BasicAuth(); s.Password != "" && s.Password != pass {
	// 		c.Response().Header().Set("WWW-Authenticate", `Basic realm="convox"`)
	// 		return stdapi.Errorf(401, "invalid authentication")
	// 	}
	// 	return nil
	// })

	s.Subrouter("/", func(auth *stdapi.Router) {
		auth.Use(s.authenticate)

		auth.Route("GET", "/auth", func(c *stdapi.Context) error { return c.RenderOK() })

		// auth.Route("GET", "/v2/{path:.*}", s.RegistryProxy)

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
			// Audit: stash the verified JWT subject so contextFrom can thread it
			// into the provider context for ContextActor / EventSend "actor"
			// derivation. Basic-auth callers (rack-password) take a separate
			// path covered by the SetReadWriteRole branch.
			c.Set(structs.ConvoxJwtUserParam, data.User)
		} else {
			if s.Password != "" && s.Password != pass {
				c.Response().Header().Set("WWW-Authenticate", `Basic realm="convox"`)
				return stdapi.Errorf(http.StatusUnauthorized, "invalid authentication")
			}
			SetReadWriteRole(c)
			// Audit: basic-auth callers (rack-password) have no JWT identity, so
			// stash the literal "rack-password" sentinel for ContextActor /
			// EventSend audit-event derivation. The literal is uniquely greppable
			// (no other "rack-password" string in the rack source path) and
			// distinct from the empty-JWT-user "unknown" fallback. See D.4 spec
			// set-d4-e1-spec-v2.md §B.1.1.
			//
			// Caller-supplied actor header (Console3-driven flow). When the
			// basic-auth caller (Console3 today, possibly other clients in the
			// future) supplies a Convox-Actor / X-Convox-Actor header, use it as
			// the audit actor identity instead of the generic "rack-password"
			// sentinel. The header value flows through ContextActor() to
			// EventSend's central injection (provider/k8s/event.go), so every
			// audit event written during the request lands the user-attributed
			// actor without per-controller form-param plumbing.
			//
			// Trust model: anyone with the rack password can already do anything
			// as root; the header is a user-attribution override, not a
			// security boundary. Forged identities are no worse than calling the
			// SDK with a forged identity. The header is purely additive — pre-
			// 3.24.6 racks ignore unknown headers, so the behavior on older racks
			// is unchanged (actor = "rack-password" sentinel).
			//
			// Migration: this header path is the universal-attribution bridge
			// until 3.25.0 ships per-user JWT minting. The JWT branch above
			// takes precedence; once Console3 migrates to JWT, the header path
			// becomes dead code on Console3 paths and can be removed in a
			// 3.25.x cleanup.
			//
			// Sanitize on receive: use audit.SanitizeActor to strip C0/C1/bidi/ZW
			// characters from hostile header values. The 256-char cap matches the
			// budget_accumulator's existing convention (sanitizeAckBy is now a
			// thin wrapper over the same helper).
			//
			// Naming convention: dual-read both `Convox-Actor` (canonical, RFC
			// 6648) and `X-Convox-Actor` (legacy X- prefix) — matches the
			// existing X-Convox-TID precedent at pkg/api/helpers.go::contextFrom.
			// Canonical wins when both are present.
			//
			// Hostile-canonical fallthrough: if the canonical header sanitizes
			// to the "unknown" sentinel (i.e. the input was exclusively
			// strip-set chars like bidi overrides or zero-width characters —
			// not stripped by TrimSpace because they're not unicode-IsSpace),
			// fall through to legacy rather than silently null-routing the
			// legitimate legacy attribution. Without this, a hostile MITM /
			// browser extension / buggy proxy that injects e.g.
			// "Convox-Actor: ‮‮‮" while leaving X-Convox-Actor
			// clean would suppress the real attribution and stamp "unknown"
			// on the audit event.
			// pickActor is a local closure — its empty-return signal flows
			// directly into the canonical → legacy → "rack-password" fallback
			// chain below. Future refactor that lifts this out (e.g. for
			// reuse in a hypothetical SystemAuth middleware) needs to keep
			// the empty-string-as-fallthrough-signal contract intact.
			//
			// Sentinel coupling: the `sanitized == "unknown"` branch relies
			// on audit.SanitizeActor returning that exact string for
			// whitespace-only / strip-set-only inputs. Both pkg/audit/
			// sanitize_test.go (producer side) and pkg/api/
			// actor_header_test.go (consumer side) pin this contract — a
			// future SanitizeActor refactor that changes the empty-return
			// value would break BOTH suites.
			pickActor := func(raw string) string {
				raw = strings.TrimSpace(raw)
				if raw == "" {
					return ""
				}
				sanitized := audit.SanitizeActor(raw)
				// SanitizeActor returns "unknown" when post-strip is empty
				// (whitespace-only OR strip-set-only input). Treat as
				// no-attribution at THIS header level so the next fallback
				// (legacy header, then "rack-password") engages.
				if sanitized == "unknown" {
					return ""
				}
				return sanitized
			}
			actor := pickActor(c.Request().Header.Get("Convox-Actor"))
			if actor == "" {
				actor = pickActor(c.Request().Header.Get("X-Convox-Actor"))
			}
			if actor == "" {
				actor = "rack-password"
			}
			c.Set(structs.ConvoxJwtUserParam, actor)
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
