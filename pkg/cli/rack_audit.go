package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/audit"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("rack audit", "get the rack api audit log", RackAudit, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.StringFlag("actor", "", "only records with this actor"),
			stdcli.BoolFlag("follow", "", "keep streaming new records"),
			stdcli.StringFlag("ip", "", "only records with this ip or peer"),
			stdcli.DurationFlag("since", "", "how far back to read (default 24h)"),
			stdcli.StringFlag("token", "", "only records that used or minted this token fingerprint"),
			stdcli.StringFlag("type", "", "only records of this type: api, exec, kubernetes, token"),
		},
		Validate: stdcli.Args(0),
	})
}

func RackAudit(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	if !strings.HasPrefix(s.Version, "3.") || isVersionLessThan(s.Version, "3.26.1") {
		return fmt.Errorf("convox rack audit needs rack version 3.26.1 or later")
	}

	if !auditRecordsClientIP(c, s.Provider) {
		fmt.Fprintln(c.Writer().Stderr, "WARNING: ip is the caller's address only on AWS racks with proxy_protocol=true or router_type=contour, otherwise a node or load balancer address")
	}

	since, ok := c.Value("since").(time.Duration)
	if !ok {
		since = 24 * time.Hour
	}

	r, err := rack.SystemAuditLogs(structs.LogsOptions{Since: options.Duration(since), Follow: options.Bool(c.Bool("follow"))})
	if err != nil {
		return err
	}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		for _, prefix := range []string{"error: ", "ERROR: "} {
			if msg, ok := strings.CutPrefix(line, prefix); ok {
				return errors.New(msg)
			}
		}

		var e audit.Entry
		if json.Unmarshal([]byte(line), &e) != nil || !auditMatch(c, &e) {
			continue
		}

		fmt.Fprintln(c, auditLine(&e))
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if c.Bool("follow") {
		return nil
	}

	return logStreamErr(r)
}

func auditRecordsClientIP(c *stdcli.Context, provider string) bool {
	if provider != "aws" {
		return false
	}

	r, err := rack.Current(c)
	if err != nil {
		return false
	}

	ps, err := r.Parameters()
	if err != nil {
		return false
	}

	return ps["proxy_protocol"] == "true" || ps["router_type"] == "contour"
}

func auditMatch(c *stdcli.Context, e *audit.Entry) bool {
	if v := c.String("type"); v != "" && v != e.Type {
		return false
	}
	if v := c.String("actor"); v != "" && v != e.Actor {
		return false
	}
	if v := c.String("ip"); v != "" && v != e.IP && v != e.Peer {
		return false
	}
	if v := c.String("token"); v != "" && v != e.Token && v != e.Minted {
		return false
	}

	return true
}

func auditLine(e *audit.Entry) string {
	status := "-"
	if e.Status != 0 {
		status = strconv.Itoa(e.Status)
	}

	var b strings.Builder

	fmt.Fprintf(&b, "%s %s %s %s %q auth=%s", e.Time, e.Type, status, e.Method, e.Path, e.Auth)
	if e.Role != "" {
		fmt.Fprintf(&b, " role=%s", e.Role)
	}
	if e.Actor != "" {
		fmt.Fprintf(&b, " actor=%q", e.Actor)
		if e.Auth == "password" && e.Actor != "rack-password" {
			b.WriteString(" (claimed)")
		}
	}
	if e.Token != "" {
		fmt.Fprintf(&b, " token=%s", e.Token)
	}
	fmt.Fprintf(&b, " ip=%s peer=%s", e.IP, e.Peer)
	if e.TID != "" {
		fmt.Fprintf(&b, " tid=%q", e.TID)
	}
	if e.Pid != "" {
		fmt.Fprintf(&b, " pid=%s", e.Pid)
	}
	if e.Command != "" {
		fmt.Fprintf(&b, " command=%q", e.Command)
	}
	if e.Minted != "" {
		fmt.Fprintf(&b, " minted=%s granted=%s hours=%d", e.Minted, e.Granted, e.Hours)
	}

	return b.String()
}
