package cli

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestRouterShutdownBudget(t *testing.T) {
	const nginxPath = "../../terraform/router/nginx/main.tf"
	const awsPath = "../../terraform/router/aws/main.tf"
	const key = `"worker-shutdown-timeout"`
	const override = "local.additional_config_map"

	comment := regexp.MustCompile(`(?m)^[ \t]*(#|//).*$`)
	resource := func(path, kind, name string) string {
		t.Helper()
		for _, block := range parseHCLBody(t, path).Blocks {
			if block.Type == "resource" && len(block.Labels) == 2 && block.Labels[0] == kind && block.Labels[1] == name {
				return comment.ReplaceAllString(string(block.Range().SliceBytes(mustReadFile(t, path))), "")
			}
		}
		t.Fatalf("%s: %s.%s not found", path, kind, name)
		return ""
	}

	seconds := func(where, src, pattern string, absent int) int {
		t.Helper()
		matches := regexp.MustCompile(pattern).FindAllStringSubmatch(src, -1)
		if len(matches) == 0 && absent >= 0 {
			return absent
		}
		if len(matches) != 1 {
			t.Fatalf("%s: %d matches for %s; want 1", where, len(matches), pattern)
		}
		n, err := strconv.Atoi(matches[0][1])
		if err != nil {
			t.Fatalf("%s: %v", where, err)
		}
		return n
	}

	type budget struct{ grace, wait int }
	var budgets []budget
	for _, name := range []string{"ingress-nginx", "ingress-nginx-internal"} {
		src := resource(nginxPath, "kubernetes_deployment", name)
		if strings.Contains(src, `"--shutdown-grace-period`) {
			t.Errorf("%s passes --shutdown-grace-period, which stops endpoint updates while it waits; the wait belongs in the preStop hook", name)
		}
		budgets = append(budgets, budget{
			grace: seconds(name, src, `(?m)^\s*termination_grace_period_seconds\s*=\s*(\d+)\s*$`, -1),
			wait: seconds(name, src, `pre_stop\s*\{\s*exec\s*\{\s*command\s*=\s*\["/bin/sh", "-c", "sleep (\d+); exec /wait-shutdown"\]`, -1) +
				seconds(name, src, `"--post-shutdown-grace-period=(\d+)"`, 10),
		})
	}
	if budgets[0] != budgets[1] {
		t.Fatalf("ingress-nginx %+v and ingress-nginx-internal %+v differ", budgets[0], budgets[1])
	}

	var drains []int
	for _, cm := range [][2]string{{nginxPath, "nginx-configuration"}, {awsPath, "nginx-configuration"}, {awsPath, "nginx-internal-configuration"}} {
		where := cm[0] + " " + cm[1]
		src := resource(cm[0], "kubernetes_config_map", cm[1])
		if n := strings.Count(src, key); n != 1 {
			t.Fatalf("%s: %s appears %d times; want 1", where, key, n)
		}
		drains = append(drains, seconds(where, src, key+`\s*=\s*"(\d+)s"`, -1))
		at := strings.Index(src, override)
		if (at < 0 && cm[1] == "nginx-configuration") || (at >= 0 && strings.Index(src, key) > at) {
			t.Errorf("%s: %s has to come before %s so nginx_additional_config can override it", where, key, override)
		}
	}
	for _, drain := range drains {
		if drain != drains[0] {
			t.Fatalf("worker-shutdown-timeout differs between config maps: %v", drains)
		}
	}

	grace, wait, drain := budgets[0].grace, budgets[0].wait, drains[0]
	if wait+drain+30 > grace {
		t.Errorf("%ds of waits and a %ds drain leave under 30s of the %ds pod grace", wait, drain, grace)
	}
	if grace > 600 {
		t.Errorf("pod grace %ds is above the 600s a cluster autoscaler gives an evicted pod", grace)
	}
}
