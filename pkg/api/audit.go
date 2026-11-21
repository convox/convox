package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/convox/convox/pkg/audit"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/gorilla/websocket"
)

const (
	auditEntryParam   = "AUDIT_ENTRY"
	auditWrittenParam = "AUDIT_WRITTEN"
)

var auditTypes = map[string]string{
	"FilesDelete":            "exec",
	"FilesDownload":          "exec",
	"FilesUpload":            "exec",
	"InstanceShell":          "exec",
	"ProcessExec":            "exec",
	"ProcessRun":             "exec",
	"Proxy":                  "exec",
	"ResourceConsole":        "exec",
	"SystemJwtSignKeyRotate": "token",
	"SystemJwtToken":         "token",
}

var (
	auditRecordStart = []byte(`{"` + audit.Marker + `":`)
	auditStreamStart = []byte("service/api/")
	logErrorStart    = []byte("error: ")
)

func (s *Server) audit(next stdapi.HandlerFunc) stdapi.HandlerFunc {
	return func(c *stdapi.Context) (err error) {
		e := audit.NewEntry(c.Request())
		e.Op = c.Name()
		e.Type = auditTypes[e.Op]
		if e.Type == "" {
			e.Type = "api"
		}
		e.ID, _ = c.Context().Value("request.id").(string)
		e.TID = requestTID(c)
		c.Set(auditEntryParam, e)

		defer func() {
			if p := recover(); p != nil {
				auditWrite(c, e, http.StatusInternalServerError)
				panic(p)
			}

			if c.Get(auditWrittenParam) == nil {
				auditWrite(c, e, auditStatus(c, err))
				return
			}

			var ae stdapi.Error
			if errors.As(err, &ae) {
				auditWrite(c, e, ae.Code())
			}
		}()

		return next(c)
	}
}

func (s *Server) auditSession(next stdapi.HandlerFunc) stdapi.HandlerFunc {
	return func(c *stdapi.Context) error {
		r := c.Request()
		if r.Method == http.MethodGet && websocket.IsWebSocketUpgrade(r) {
			auditWrite(c, auditEntry(c), http.StatusSwitchingProtocols)
			c.Set(auditWrittenParam, true)
		}

		return next(c)
	}
}

func auditEntry(c *stdapi.Context) *audit.Entry {
	if e, ok := c.Get(auditEntryParam).(*audit.Entry); ok {
		return e
	}

	return &audit.Entry{}
}

func auditWrite(c *stdapi.Context, e *audit.Entry, status int) {
	e.Time = ""
	e.Status = status

	if role, ok := c.Get(structs.ConvoxRoleParam).(string); ok && role != "" {
		e.Role = role
		e.Actor, _ = c.Get(structs.ConvoxJwtUserParam).(string)

		switch e.Op {
		case "ProcessExec":
			e.Command = c.Value("command")
		case "InstanceShell", "ProcessRun":
			e.Command = c.Request().Header.Get("Command")
		}
	}

	audit.Write(e)
}

func auditStatus(c *stdapi.Context, err error) int {
	var ae stdapi.Error

	switch {
	case errors.As(err, &ae):
		return ae.Code()
	case err != nil:
		return http.StatusInternalServerError
	case c.Response().Code() != 0:
		return c.Response().Code()
	default:
		return http.StatusOK
	}
}

func auditMessage(line []byte, prefix bool) []byte {
	msg := line

	if prefix {
		parts := bytes.SplitN(line, []byte(" "), 3)
		if len(parts) != 3 || !bytes.HasPrefix(parts[1], auditStreamStart) {
			return nil
		}
		msg = parts[2]
	}

	if !bytes.HasPrefix(msg, auditRecordStart) {
		return nil
	}

	return msg
}

func dropAuditRecords(prefix bool) func([]byte) []byte {
	return func(line []byte) []byte {
		if auditMessage(line, prefix) != nil {
			return nil
		}

		return line
	}
}

func keepAuditRecords(line []byte) []byte {
	if bytes.HasPrefix(line, logErrorStart) {
		return line
	}

	if msg := auditMessage(line, true); msg != nil && json.Valid(msg) {
		return msg
	}

	return nil
}

type logLines struct {
	w    io.Writer
	keep func([]byte) []byte
	buf  []byte
}

func (l *logLines) Write(p []byte) (int, error) {
	l.buf = append(l.buf, p...)

	var out []byte
	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i < 0 {
			break
		}
		out = append(out, l.keep(l.buf[:i+1])...)
		l.buf = l.buf[i+1:]
	}

	if len(out) > 0 {
		if _, err := l.w.Write(out); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func (l *logLines) flush() error {
	out := l.keep(l.buf)
	l.buf = nil

	if len(out) == 0 {
		return nil
	}

	_, err := l.w.Write(out)

	return err
}
