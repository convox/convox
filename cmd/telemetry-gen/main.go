package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

var providers = []string{"aws", "azure", "do", "gcp", "local", "metal"}

// telemetryOmit are params the generator drops from both emitted maps.
var telemetryOmit = map[string]bool{
	"access_id":        true,
	"secret_key":       true,
	"token":            true,
	"private_eks_host": true,
	"private_eks_user": true,
}

// telemetryRedact are params kept in the maps and covered by both redaction layers.
var telemetryRedact = map[string]bool{
	"docker_hub_password": true,
	"webhook_signing_key": true,
	"private_eks_pass":    true,
	"prometheus_url":      true,
}

// telemetryBenign are coverage-pattern params that are not secrets.
var telemetryBenign = map[string]bool{
	"imds_http_tokens":            true,
	"private_subnets_ids":         true,
	"karpenter_build_imds_tokens": true,
	"private_api":                 true,
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: telemetry-gen <provider>|verify")
		os.Exit(2)
	}
	provider := os.Args[1]

	if provider == "verify" {
		if err := VerifyAll("."); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	var b bytes.Buffer
	if err := renderTelemetry(&b, ".", provider); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join("terraform", "system", provider, "telemetry.tf"), b.Bytes(), 0644); err != nil { //nolint:gosec // generated terraform source, world-readable like its siblings
		panic(err)
	}
}

func VerifyAll(baseDir string) error {
	for _, p := range providers {
		data, err := os.ReadFile(filepath.Join(baseDir, "terraform", "system", p, "telemetry.tf"))
		if err != nil {
			return err
		}

		var b bytes.Buffer
		if err := renderTelemetry(&b, baseDir, p); err != nil {
			return err
		}

		if b.String() != string(data) {
			fmt.Printf("mismatch found for %s\n", p)
			fmt.Println("+++++++++++++++++++++++")
			fmt.Println(b.String())
			fmt.Println("-----------------------")
			fmt.Println(string(data))
			fmt.Println("+++++++++++++++++++++++")
			return fmt.Errorf("mismatch")
		}
	}
	return nil
}

// computeMaps returns the value and default maps for a provider, minus telemetryOmit,
// erroring if a credential-shaped variable is not triaged into a bucket.
func computeMaps(baseDir, provider string) (map[string]string, map[string]interface{}, error) {
	module, diags := tfconfig.LoadModule(filepath.Join(baseDir, "terraform", "system", provider))
	if diags.Err() != nil {
		return nil, nil, diags.Err()
	}

	names := make([]string, 0, len(module.Variables))
	for _, v := range module.Variables {
		names = append(names, v.Name)
	}
	if un := unbucketedParamNames(names, telemetryOmit, telemetryRedact, telemetryBenign); len(un) > 0 {
		return nil, nil, fmt.Errorf("%s: coverage-pattern params not triaged (add to omit/redact/benign): %s", provider, strings.Join(un, ", "))
	}

	varMap := map[string]string{}
	defaultMap := map[string]interface{}{}
	for _, v := range module.Variables {
		if telemetryOmit[v.Name] {
			continue
		}
		varMap[v.Name] = fmt.Sprintf("var.%s", v.Name)
		if v.Default == nil {
			defaultMap[v.Name] = ""
		} else {
			defaultMap[v.Name] = v.Default
		}
	}
	return varMap, defaultMap, nil
}

func renderTelemetry(w io.Writer, baseDir, provider string) error {
	varMap, defaultMap, err := computeMaps(baseDir, provider)
	if err != nil {
		return err
	}

	tp, err := template.New("").ParseFiles(filepath.Join(baseDir, "cmd", "telemetry-gen", "telemetry.tf.tmpl"))
	if err != nil {
		return err
	}

	var raw bytes.Buffer
	if err := tp.ExecuteTemplate(&raw, "telemetry", map[string]interface{}{
		"Provider":   provider,
		"VarMap":     varMap,
		"DefaultMap": defaultMap,
	}); err != nil {
		return err
	}

	_, err = w.Write(hclwrite.Format(raw.Bytes()))
	return err
}

// unbucketedParamNames returns credential-shaped variable names not present in any bucket.
func unbucketedParamNames(names []string, omit, redact, benign map[string]bool) []string {
	var out []string
	for _, n := range names {
		l := strings.ToLower(n)
		flagged := strings.Contains(l, "password") ||
			strings.Contains(l, "secret") ||
			strings.Contains(l, "token") ||
			strings.Contains(l, "_pass") ||
			strings.Contains(l, "access_id") ||
			strings.HasSuffix(l, "_key") ||
			strings.HasPrefix(l, "private_")
		if !flagged || omit[n] || redact[n] || benign[n] {
			continue
		}
		out = append(out, n)
	}
	return out
}
