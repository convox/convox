package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("deploy-debug", "diagnose deploy failures and non-ready processes", DeployDebug, stdcli.CommandOptions{
		Flags: append(stdcli.OptionFlags(structs.AppDiagnoseOptions{}),
			flagApp,
			flagRack,
			flagWatchInterval,
			stdcli.StringFlag("output", "o", "output format: terminal, summary, json"),
			stdcli.BoolFlag("no-events", "", "skip cluster events"),
			stdcli.BoolFlag("no-previous", "", "skip previous container crash logs"),
		),
		Validate: stdcli.Args(0),
	})
}

func DeployDebug(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.AppDiagnoseOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	if c.Bool("no-events") {
		f := false
		opts.Events = &f
	}
	if c.Bool("no-previous") {
		f := false
		opts.Previous = &f
	}

	report, err := rack.AppDiagnose(app(c), opts)
	if err != nil {
		return err
	}

	switch c.String("output") {
	case "json":
		return renderDiagnoseJSON(c, report)
	case "summary":
		return renderDiagnoseSummary(c, report)
	default:
		return renderDiagnoseTerminal(c, report)
	}
}

// renderDiagnoseJSON outputs the full report as formatted JSON.
func renderDiagnoseJSON(c *stdcli.Context, report *structs.AppDiagnosticReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	c.Writef("%s\n", string(data))
	return nil
}

// renderDiagnoseSummary outputs a compact table of non-healthy processes.
func renderDiagnoseSummary(c *stdcli.Context, report *structs.AppDiagnosticReport) error {
	// Service status table
	if report.Overview != nil && len(report.Overview.Services) > 0 {
		t := c.Table("SERVICE", "DESIRED", "READY", "STATUS")
		for _, svc := range report.Overview.Services {
			status := svc.Status
			if svc.StallReason != "" {
				status = fmt.Sprintf("%s *", status)
			}
			t.AddRow(svc.Name, fmt.Sprintf("%d", svc.DesiredReplicas), fmt.Sprintf("%d", svc.ReadyReplicas), status)
		}
		t.Print()
		c.Writef("\n")
	}

	// Pod summary table
	if len(report.Pods) > 0 {
		t := c.Table("PROCESS", "SERVICE", "STATUS", "READY", "RESTARTS", "AGE", "HINT")
		for _, pod := range report.Pods {
			hint := pod.Hint
			if len(hint) > 60 {
				hint = hint[:57] + "..."
			}
			t.AddRow(
				pod.Name,
				pod.Service,
				coalesce(pod.StateDetail, pod.Phase),
				pod.Ready,
				fmt.Sprintf("%d", pod.Restarts),
				formatAge(pod.AgeSeconds),
				hint,
			)
		}
		t.Print()
	} else if report.Summary != nil && report.Summary.Unhealthy == 0 && report.Summary.NotReady == 0 {
		c.Writef("All %d processes healthy.\n", report.Summary.Total)
	} else {
		c.Writef("No processes found.\n")
	}

	return nil
}

// renderDiagnoseTerminal outputs the full color diagnostic report.
func renderDiagnoseTerminal(c *stdcli.Context, report *structs.AppDiagnosticReport) error {
	// Header
	c.Writef("\n")
	c.Writef("<h1>Deploy Diagnostics:</h1> <app>%s</app> on <rack>%s</rack>\n", report.App, report.Rack)
	c.Writef("<h2>Namespace:</h2> %s\n", report.Namespace)
	c.Writef("<h2>Time:</h2>      %s\n", report.Timestamp.Format(time.RFC3339))
	c.Writef("\n")

	// Overview section
	if report.Overview != nil {
		renderOverviewSection(c, report.Overview)
	}

	// Init container section
	if len(report.InitPods) > 0 {
		renderInitSection(c, report.InitPods)
	}

	// Pod diagnostics
	renderPodsSection(c, report)

	// Summary footer
	if report.Summary != nil {
		c.Writef("\n")
		renderSummaryFooter(c, report.Summary)
	}

	// Legend
	c.Writef("\n<h2>Legend:</h2> <fail>●</fail> unhealthy  <service>●</service> not-ready  <process>●</process> new  <ok>●</ok> healthy\n")
	c.Writef("\n")

	return nil
}

func renderOverviewSection(c *stdcli.Context, overview *structs.DiagnosticOverview) {
	c.Writef("<h1>--- Service Status ---</h1>\n")

	if len(overview.Services) == 0 {
		c.Writef("  No services found.\n")
	} else {
		for _, svc := range overview.Services {
			icon := statusIcon(svc.Status)
			agentTag := ""
			if svc.Agent {
				agentTag = " <h2>(agent)</h2>"
			}

			pword := "processes"
			if svc.DesiredReplicas == 1 {
				pword = "process"
			}

			c.Writef("  %s <service>%s</service>%s  %d/%d %s ready  %s\n",
				icon, svc.Name, agentTag,
				svc.ReadyReplicas, svc.DesiredReplicas, pword,
				statusLabel(svc.Status))

			if svc.StallReason != "" {
				c.Writef("      <fail>%s</fail>\n", svc.StallReason)
			}
		}
	}

	// Warning events
	if len(overview.Events) > 0 {
		c.Writef("\n  <h1>Warning Events:</h1>\n")
		limit := len(overview.Events)
		if limit > 15 {
			limit = 15
		}
		for _, ev := range overview.Events[:limit] {
			ago := formatAge(int(time.Since(ev.Timestamp).Seconds()))
			c.Writef("    [%s ago] %s on %s: %s\n", ago, ev.Reason, ev.Object, ev.Message)
			if ev.Hint != "" {
				c.Writef("             <h2>%s</h2>\n", ev.Hint)
			}
		}
		if len(overview.Events) > 15 {
			c.Writef("    ... and %d more events\n", len(overview.Events)-15)
		}
	}
	c.Writef("\n")
}

func renderInitSection(c *stdcli.Context, initPods []structs.DiagnosticInitPod) {
	c.Writef("<h1>--- Init Containers ---</h1>\n")

	for _, ip := range initPods {
		c.Writef("  <fail>●</fail> <process>%s</process> (service: <service>%s</service>)\n", ip.Name, ip.Service)
		for _, ic := range ip.InitContainers {
			c.Writef("    init-container/%s: %s\n", ic.Name, ic.State)
			if ic.Logs != "" {
				renderLogBlock(c, "Init Container Logs", ic.Logs, 6)
			}
		}
	}
	c.Writef("\n")
}

func renderPodsSection(c *stdcli.Context, report *structs.AppDiagnosticReport) {
	c.Writef("<h1>--- Processes ---</h1>\n")

	if len(report.Pods) == 0 {
		if report.Summary != nil && report.Summary.Total > 0 {
			c.Writef("  All %d processes healthy -- no issues detected.\n", report.Summary.Total)
		} else {
			c.Writef("  No processes found.\n")
		}
		return
	}

	for _, pod := range report.Pods {
		icon := classificationIcon(pod.Classification)

		c.Writef("\n  %s <service>%s</service>  %s\n", icon, pod.Service, classificationLabel(pod.Classification))
		c.Writef("    <h2>process:</h2> <process>%s</process>\n", pod.Name)
		c.Writef("    <h2>state:</h2> %s    <h2>ready:</h2> %s    <h2>age:</h2> %s    <h2>restarts:</h2> %d\n",
			pod.Phase, pod.Ready, formatAge(pod.AgeSeconds), pod.Restarts)

		if pod.StateDetail != "" {
			c.Writef("    <h2>detail:</h2>  %s\n", stateDetailColor(pod.Classification, pod.StateDetail))
		}

		if pod.Hint != "" {
			c.Writef("    <h1>hint:</h1>    %s\n", pod.Hint)
		}

		// Logs
		if pod.Logs != "" {
			renderLogBlock(c, fmt.Sprintf("Current Logs (last %d lines)", logLineCount(pod.Logs)), pod.Logs, 4)
		}

		// Previous crash logs
		if pod.PreviousLogs != "" {
			renderLogBlock(c, "Previous Container Logs", pod.PreviousLogs, 4)
		}

		// Events
		if len(pod.Events) > 0 {
			c.Writef("\n    <h1>--- Events ---</h1>\n")
			for _, ev := range pod.Events {
				ago := formatAge(int(time.Since(ev.Timestamp).Seconds()))
				c.Writef("    %s ago  %-8s %-20s %s\n", ago, ev.Type, ev.Reason, ev.Message)
			}
		}
	}
}

func renderSummaryFooter(c *stdcli.Context, summary *structs.DiagnosticSummary) {
	c.Writef("<h1>Summary:</h1> %d total", summary.Total)
	if summary.Unhealthy > 0 {
		c.Writef("  <fail>%d unhealthy</fail>", summary.Unhealthy)
	}
	if summary.NotReady > 0 {
		c.Writef("  <service>%d not-ready</service>", summary.NotReady)
	}
	if summary.New > 0 {
		c.Writef("  <process>%d new</process>", summary.New)
	}
	if summary.Healthy > 0 {
		c.Writef("  <ok>%d healthy</ok>", summary.Healthy)
	}
	c.Writef("\n")
}

func renderLogBlock(c *stdcli.Context, title, logs string, indent int) {
	prefix := strings.Repeat(" ", indent)
	c.Writef("\n%s<h2>--- %s ---</h2>\n", prefix, title)

	lines := strings.Split(strings.TrimRight(logs, "\n"), "\n")
	maxLines := 50
	start := 0
	if len(lines) > maxLines {
		start = len(lines) - maxLines
		c.Writef("%s<h2>... (%d lines truncated) ...</h2>\n", prefix, start)
	}
	for _, line := range lines[start:] {
		c.Writef("%s%s\n", prefix, line)
	}
}

// Helper functions

func statusIcon(status string) string {
	switch status {
	case "running":
		return "<ok>●</ok>"
	case "stalled":
		return "<fail>●</fail>"
	case "deploying":
		return "<service>●</service>"
	case "scaled-down":
		return "<h2>○</h2>"
	default:
		return "<h2>●</h2>"
	}
}

func statusLabel(status string) string {
	switch status {
	case "running":
		return "<ok>RUNNING</ok>"
	case "stalled":
		return "<fail>STALLED</fail>"
	case "deploying":
		return "<service>DEPLOYING</service>"
	case "scaled-down":
		return "<h2>SCALED DOWN</h2>"
	default:
		return status
	}
}

func classificationIcon(classification string) string {
	switch classification {
	case "unhealthy":
		return "<fail>●</fail>"
	case "not-ready":
		return "<service>●</service>"
	case "new":
		return "<process>●</process>"
	case "healthy":
		return "<ok>●</ok>"
	default:
		return "●"
	}
}

func classificationLabel(classification string) string {
	switch classification {
	case "unhealthy":
		return "<fail>unhealthy</fail>"
	case "not-ready":
		return "<service>not-ready</service>"
	case "new":
		return "<process>new</process>"
	case "healthy":
		return "<ok>healthy</ok>"
	default:
		return classification
	}
}

func stateDetailColor(classification, detail string) string {
	switch classification {
	case "unhealthy":
		return fmt.Sprintf("<fail>%s</fail>", detail)
	case "not-ready":
		return fmt.Sprintf("<service>%s</service>", detail)
	default:
		return detail
	}
}

func formatAge(seconds int) string {
	if seconds < 0 {
		return "unknown"
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	return fmt.Sprintf("%dd", seconds/86400)
}

func logLineCount(logs string) int {
	if logs == "" {
		return 0
	}
	return len(strings.Split(strings.TrimRight(logs, "\n"), "\n"))
}
