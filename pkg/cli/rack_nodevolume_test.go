package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const nodeFilesDir = "../../terraform/cluster/aws/files/"

const wantUserDataDefault = `MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==MYBOUNDARY=="

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
echo "CONVOX MANAGED USER DATA SCRIPT"

echo "USER PROVIDED USER DATA SCRIPT"
echo hello



--==MYBOUNDARY==--`

const wantUserDataRegistry = `MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="==MYBOUNDARY=="

--==MYBOUNDARY==
Content-Type: text/x-shellscript; charset="us-ascii"

#!/bin/bash
echo "CONVOX MANAGED USER DATA SCRIPT"

echo "USER PROVIDED USER DATA SCRIPT"
echo hello



--==MYBOUNDARY==
Content-Type: application/node.eks.aws

apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  kubelet:
    config:
      registryPullQPS: 20
      registryBurst: 40

--==MYBOUNDARY==--`

const wantAL2023Default = `MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="//"

--//
Content-Type: application/node.eks.aws

---
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    apiServerEndpoint: https://example.eks.amazonaws.com
    certificateAuthority: Q0E=
    name: cluster
    cidr: 10.1.0.0/16
  kubelet:
    config:
      clusterDNS:
      - 10.100.0.10
    maxPodsExpression: "((default_enis - 1) * (ips_per_eni - 1)) + 2"
    flags:
    - "--node-labels=eks.amazonaws.com/nodegroup=rack-additional-0-abc"
--//

Content-Type: text/x-shellscript

---
#!/bin/bash
# Custom user data script
echo hello

--//--
`

const wantAL2023Registry = `MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="//"

--//
Content-Type: application/node.eks.aws

---
apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  cluster:
    apiServerEndpoint: https://example.eks.amazonaws.com
    certificateAuthority: Q0E=
    name: cluster
    cidr: 10.1.0.0/16
  kubelet:
    config:
      clusterDNS:
      - 10.100.0.10
      registryPullQPS: 20
      registryBurst: 40
    maxPodsExpression: "((default_enis - 1) * (ips_per_eni - 1)) + 2"
    flags:
    - "--node-labels=eks.amazonaws.com/nodegroup=rack-additional-0-abc"
--//

Content-Type: text/x-shellscript

---
#!/bin/bash
# Custom user data script
echo hello

--//--
`

func renderNodeUserData(t *testing.T, name string, registrySet, fastImagePull bool) string {
	t.Helper()
	qps, burst := int64(5), int64(10)
	if registrySet {
		qps, burst = 20, 40
	}
	vars := map[string]cty.Value{
		"kubelet_registry_pull_qps": cty.NumberIntVal(qps),
		"kubelet_registry_burst":    cty.NumberIntVal(burst),
		"kubelet_registry_set":      cty.BoolVal(registrySet),
		"fast_image_pull":           cty.BoolVal(fastImagePull),
		"user_data":                 cty.StringVal("echo hello"),
		"user_data_script_file":     cty.StringVal(""),
		"api_server_endpoint":       cty.StringVal("https://example.eks.amazonaws.com"),
		"api_server_ca":             cty.StringVal("Q0E="),
		"name":                      cty.StringVal("cluster"),
		"cidr":                      cty.StringVal("10.1.0.0/16"),
		"cluster_dns":               cty.StringVal("10.100.0.10"),
		"node_labels":               cty.StringVal("eks.amazonaws.com/nodegroup=rack-additional-0-abc"),
	}

	path := nodeFilesDir + name
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	expr, diags := hclsyntax.ParseTemplate(src, path, hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parse %s: %v", path, diags)
	}
	v, diags := expr.Value(&hcl.EvalContext{Variables: vars})
	if diags.HasErrors() {
		t.Fatalf("render %s: %v", path, diags)
	}
	return v.AsString()
}

// TestNodeUserDataUnchangedWithoutFastImagePull pins the rendered bytes of both
// node userData templates while fast_image_pull is off. spec.userData is inside
// the Karpenter EC2NodeClass drift hash and the launch template user_data is
// base64 of the same text, so one added or dropped byte rolls every node group
// and every pool on a rack that never set the parameter.
func TestNodeUserDataUnchangedWithoutFastImagePull(t *testing.T) {
	for _, tc := range []struct {
		name        string
		template    string
		registrySet bool
		want        string
	}{
		{"user-data registry default", "custom_user_data.sh", false, wantUserDataDefault},
		{"user-data registry off-default", "custom_user_data.sh", true, wantUserDataRegistry},
		{"al2023 registry default", "custom_ami_userdata_al2023.sh", false, wantAL2023Default},
		{"al2023 registry off-default", "custom_ami_userdata_al2023.sh", true, wantAL2023Registry},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderNodeUserData(t, tc.template, tc.registrySet, false); got != tc.want {
				t.Errorf("render changed\n--- want ---\n%s\n--- got ---\n%s", tc.want, got)
			}
		})
	}
}

func TestNodeUserDataFastImagePull(t *testing.T) {
	const gate = "featureGates:\n    FastImagePull: true"

	for _, tc := range []struct {
		template    string
		registrySet bool
		wantKubelet bool
	}{
		{"custom_user_data.sh", false, false},
		{"custom_user_data.sh", true, true},
		{"custom_ami_userdata_al2023.sh", false, false},
		{"custom_ami_userdata_al2023.sh", true, true},
	} {
		name := tc.template
		if tc.registrySet {
			name += " with registry params"
		}
		t.Run(name, func(t *testing.T) {
			got := renderNodeUserData(t, tc.template, tc.registrySet, true)
			if !strings.Contains(got, gate) {
				t.Fatalf("missing feature gate stanza:\n%s", got)
			}
			if strings.Contains(got, "registryPullQPS") != tc.wantKubelet {
				t.Errorf("registry keys present=%v, want %v:\n%s", !tc.wantKubelet, tc.wantKubelet, got)
			}
			if !strings.Contains(got, "NodeConfig") {
				t.Errorf("no NodeConfig part emitted:\n%s", got)
			}
		})
	}

	// The MIME part must not be emitted at all when neither key is set.
	if got := renderNodeUserData(t, "custom_user_data.sh", false, false); strings.Contains(got, "NodeConfig") {
		t.Errorf("NodeConfig part emitted with nothing to configure:\n%s", got)
	}
}

// TestNodeUserDataTemplateArgsComplete asserts every templatefile() call for the
// two node userData templates supplies every name the template references. CI
// runs terraform validate with continue-on-error and validate does not evaluate
// templatefile, so a missing key first surfaces at apply on a live rack.
func TestNodeUserDataTemplateArgsComplete(t *testing.T) {
	callers := map[string]string{
		"main.tf":        readTestFile(t, "../../terraform/cluster/aws/main.tf"),
		"node_groups.tf": readTestFile(t, "../../terraform/cluster/aws/node_groups.tf"),
	}

	for _, tc := range []struct {
		template string
		calls    int
	}{
		{"custom_user_data.sh", 3},
		{"custom_ami_userdata_al2023.sh", 2},
	} {
		t.Run(tc.template, func(t *testing.T) {
			want := templateReferences(readTestFile(t, nodeFilesDir+tc.template))
			if len(want) == 0 {
				t.Fatalf("parsed no references out of %s", tc.template)
			}

			callRe := regexp.MustCompile(`(?s)templatefile\("\$\{path\.module\}/files/` +
				regexp.QuoteMeta(tc.template) + `",\s*\{(.*?)\n  \}\)`)
			argRe := regexp.MustCompile(`(?m)^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*=`)

			found := 0
			for file, src := range callers {
				for i, call := range callRe.FindAllStringSubmatch(src, -1) {
					found++
					got := map[string]bool{}
					for _, m := range argRe.FindAllStringSubmatch(call[1], -1) {
						got[m[1]] = true
					}
					var missing []string
					for name := range want {
						if !got[name] {
							missing = append(missing, name)
						}
					}
					sort.Strings(missing)
					if len(missing) > 0 {
						t.Errorf("call %d in %s does not supply %v", i+1, file, missing)
					}
				}
			}
			if found != tc.calls {
				t.Errorf("found %d templatefile calls for %s; expected %d", found, tc.template, tc.calls)
			}
		})
	}
}

// templateReferences returns the bare identifiers an HCL template interpolates,
// skipping directive keywords and function names.
func templateReferences(tpl string) map[string]bool {
	keyword := map[string]bool{
		"for": true, "endfor": true, "if": true, "else": true, "endif": true,
		"in": true, "null": true, "true": true, "false": true,
	}
	ident := regexp.MustCompile(`(\.?)([a-zA-Z_][a-zA-Z0-9_]*)(\s*\()?`)
	refs := map[string]bool{}
	for _, body := range regexp.MustCompile(`[$%]\{([^}]*)\}`).FindAllStringSubmatch(tpl, -1) {
		for _, m := range ident.FindAllStringSubmatch(body[1], -1) {
			if m[1] == "" && m[3] == "" && !keyword[m[2]] {
				refs[m[2]] = true
			}
		}
	}
	return refs
}

func TestValidateAndMutateParams_NodeVolumeRanges(t *testing.T) {
	for _, tc := range []struct {
		params  map[string]string
		current map[string]string
		wantErr string
		warn    string
	}{
		{params: map[string]string{"node_volume_iops": "0"}},
		{params: map[string]string{"node_volume_iops": "4000"}},
		{params: map[string]string{"node_volume_iops": "80000"}},
		{params: map[string]string{"node_volume_throughput": "0"}},
		{params: map[string]string{"node_volume_throughput": "600"}},
		{params: map[string]string{"node_volume_throughput": "1000"}},
		{params: map[string]string{"node_volume_iops": "2999"}, wantErr: "3000"},
		{params: map[string]string{"node_volume_iops": "80001"}, wantErr: "80000"},
		{params: map[string]string{"karpenter_node_volume_iops": "100"}, wantErr: "3000"},
		{params: map[string]string{"node_volume_throughput": "124"}, wantErr: "125"},
		{params: map[string]string{"node_volume_throughput": "2000"}, wantErr: "1000"},
		{params: map[string]string{"karpenter_node_volume_throughput": "50"}, wantErr: "125"},
		{params: map[string]string{"node_volume_iops": "abc"}, wantErr: "non-negative integer"},
		{params: map[string]string{"node_volume_throughput": "-1"}, wantErr: "non-negative integer"},

		// gp3 is the only volume type AWS accepts either field on.
		{params: map[string]string{"karpenter_node_volume_throughput": "600"}},
		{
			params:  map[string]string{"karpenter_node_volume_throughput": "600"},
			current: map[string]string{"karpenter_node_volume_type": "io2"},
			wantErr: "gp3",
		},
		// Switching the type away from gp3 voids a stored value. That warns rather than
		// refusing, because the value still reaches a custom nodepool that runs gp3, and
		// the warning is scoped to the call that moves the type so it does not repeat.
		{
			params:  map[string]string{"karpenter_node_volume_type": "io2"},
			current: map[string]string{"karpenter_node_volume_iops": "4000"},
			warn:    "no longer applies",
		},
		{
			params:  map[string]string{"node_disk": "100"},
			current: map[string]string{"karpenter_node_volume_type": "io2", "karpenter_node_volume_iops": "4000"},
		},
		{
			params:  map[string]string{"karpenter_node_volume_iops": "4000", "karpenter_node_volume_type": "gp2"},
			wantErr: "gp3",
		},
		{
			params:  map[string]string{"karpenter_node_volume_throughput": "600", "karpenter_node_volume_type": "gp3"},
			current: map[string]string{"karpenter_node_volume_type": "io2"},
		},
		// The rack-wide value is inherited but never rendered onto a non-gp3
		// Karpenter volume, so it is not the operator's mistake to refuse.
		{
			params:  map[string]string{"node_volume_throughput": "600"},
			current: map[string]string{"karpenter_node_volume_type": "io2"},
		},
		// Non-AWS providers never see these params.
		{params: map[string]string{"node_volume_throughput": "50"}, current: map[string]string{"provider": "gcp"}},
	} {
		name := fmt.Sprint(tc.params)
		t.Run(name, func(t *testing.T) {
			provider := "aws"
			current := tc.current
			if current["provider"] == "gcp" {
				provider, current = "gcp", map[string]string{}
			}
			var err error
			stderr := captureStderr(t, func() {
				err = validateAndMutateParams(copyParams(tc.params), provider, current, true)
			})
			switch {
			case tc.warn != "" && !strings.Contains(stderr, tc.warn):
				t.Errorf("stderr %q should carry the warning %q", stderr, tc.warn)
			case tc.warn == "" && strings.Contains(stderr, "no longer applies"):
				t.Errorf("unexpected warning on stderr: %q", stderr)
			}
			switch {
			case tc.wantErr == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Errorf("expected error containing %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Errorf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateAndMutateParams_FastImagePullThroughput(t *testing.T) {
	pools := func(entries string) string {
		return base64.StdEncoding.EncodeToString([]byte(entries))
	}

	for _, tc := range []struct {
		name    string
		params  map[string]string
		current map[string]string
		force   bool
		refused bool
		raise   string
	}{
		{name: "default throughput", params: map[string]string{"fast_image_pull_enable": "true"}, refused: true},
		{name: "non-canonical true", params: map[string]string{"fast_image_pull_enable": "1"}, refused: true},
		{name: "forced", params: map[string]string{"fast_image_pull_enable": "true"}, force: true},
		{
			name:   "throughput in the same call",
			params: map[string]string{"fast_image_pull_enable": "true", "node_volume_throughput": "600"},
		},
		{
			name:    "throughput already applied",
			params:  map[string]string{"fast_image_pull_enable": "true"},
			current: map[string]string{"node_volume_throughput": "750"},
		},
		{
			// 1000 clamps to a quarter of the gp3 baseline 3000 IOPS, still above the floor.
			name:   "throughput above the iops ceiling",
			params: map[string]string{"fast_image_pull_enable": "true", "node_volume_throughput": "1000"},
		},
		{
			name:   "iops raised alongside",
			params: map[string]string{"fast_image_pull_enable": "true", "node_volume_throughput": "1000", "node_volume_iops": "4000"},
		},
		{
			name:    "unrelated param on a rack already carrying the gate",
			params:  map[string]string{"node_disk": "100"},
			current: map[string]string{"fast_image_pull_enable": "true"},
		},
		{
			name:    "lowering throughput under an applied gate",
			params:  map[string]string{"node_volume_throughput": "125"},
			current: map[string]string{"fast_image_pull_enable": "true", "node_volume_throughput": "750"},
			refused: true,
		},
		{
			name:    "karpenter pool below the floor",
			params:  map[string]string{"fast_image_pull_enable": "true", "node_volume_throughput": "600"},
			current: map[string]string{"karpenter_enabled": "true", "karpenter_node_volume_throughput": "200"},
			refused: true,
			raise:   "karpenter_node_volume_throughput",
		},
		{
			// Every Karpenter surface inherits, so raising the rack value fixes all of them
			// and the remediation must not also name a parameter that is not set.
			name:    "karpenter enabled and inheriting the default",
			params:  map[string]string{"fast_image_pull_enable": "true"},
			current: map[string]string{"karpenter_enabled": "true"},
			refused: true,
		},
		{
			name:    "karpenter disabled, pool value ignored",
			params:  map[string]string{"fast_image_pull_enable": "true", "node_volume_throughput": "600"},
			current: map[string]string{"karpenter_node_volume_throughput": "200"},
		},
		{
			name:   "custom nodepool below the floor",
			params: map[string]string{"fast_image_pull_enable": "true", "node_volume_throughput": "600"},
			current: map[string]string{
				"karpenter_enabled":                     "true",
				"additional_karpenter_nodepools_config": pools(`[{"name":"gpu","volume_throughput":300}]`),
			},
			refused: true,
			raise:   "volume_throughput on nodepool gpu",
		},
		{
			name:   "custom nodepool inherits the rack value",
			params: map[string]string{"fast_image_pull_enable": "true", "node_volume_throughput": "600"},
			current: map[string]string{
				"karpenter_enabled":                     "true",
				"additional_karpenter_nodepools_config": pools(`[{"name":"gpu"}]`),
			},
		},
		{
			name:    "pools-only call adds a sub-floor pool under an applied gate",
			params:  map[string]string{"additional_karpenter_nodepools_config": pools(`[{"name":"gpu","volume_throughput":300}]`)},
			current: map[string]string{"fast_image_pull_enable": "true", "karpenter_enabled": "true", "node_volume_throughput": "750"},
			refused: true,
			raise:   "volume_throughput on nodepool gpu",
		},
		{
			name:   "non-gp3 custom nodepool cannot carry throughput at all",
			params: map[string]string{"fast_image_pull_enable": "true", "node_volume_throughput": "600"},
			current: map[string]string{
				"karpenter_enabled":                     "true",
				"karpenter_node_volume_throughput":      "200",
				"karpenter_node_volume_type":            "io2",
				"additional_karpenter_nodepools_config": pools(`[{"name":"gpu","volume_type":"io2"}]`),
			},
		},
		{
			// Terraform floors iops at the gp3 baseline before deriving the throughput
			// ceiling, so a stored sub-baseline value must not lower what the CLI compares.
			name:    "stored iops below the gp3 baseline",
			params:  map[string]string{"fast_image_pull_enable": "true"},
			current: map[string]string{"node_volume_iops": "500", "node_volume_throughput": "1000"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAndMutateParams(copyParams(tc.params), "aws", tc.current, tc.force)
			if tc.refused {
				if err == nil {
					t.Fatal("expected a refusal")
				}
				if !strings.Contains(err.Error(), "600 MiB/s") || !strings.Contains(err.Error(), "--force") {
					t.Errorf("refusal %q must name 600 MiB/s and --force", err)
				}
				raise := tc.raise
				if raise == "" {
					raise = "node_volume_throughput"
				}
				if !strings.Contains(err.Error(), "Raise "+raise+" to 600") {
					t.Errorf("refusal %q must tell the operator to raise exactly %s", err, raise)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestKarpenterNodePoolVolumePerformance(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }

	for _, tc := range []struct {
		pool    KarpenterNodePoolConfigParam
		wantErr string
	}{
		{pool: KarpenterNodePoolConfigParam{Name: "gpu", VolumeIops: intPtr(4000), VolumeThroughput: intPtr(600)}},
		{pool: KarpenterNodePoolConfigParam{Name: "gpu", VolumeIops: intPtr(0), VolumeThroughput: intPtr(0)}},
		{pool: KarpenterNodePoolConfigParam{Name: "gpu", VolumeIops: intPtr(100)}, wantErr: "3000"},
		{pool: KarpenterNodePoolConfigParam{Name: "gpu", VolumeThroughput: intPtr(50)}, wantErr: "125"},
		{pool: KarpenterNodePoolConfigParam{Name: "gpu", VolumeThroughput: intPtr(2000)}, wantErr: "1000"},
		{
			pool:    KarpenterNodePoolConfigParam{Name: "gpu", VolumeType: strPtr("io2"), VolumeIops: intPtr(4000)},
			wantErr: "gp3",
		},
		{
			pool:    KarpenterNodePoolConfigParam{Name: "gpu", VolumeType: strPtr("gp2"), VolumeThroughput: intPtr(600)},
			wantErr: "gp3",
		},
	} {
		t.Run(fmt.Sprint(tc.wantErr), func(t *testing.T) {
			err := tc.pool.Validate()
			switch {
			case tc.wantErr == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Errorf("expected error containing %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Errorf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestKarpenterNodePoolVolumeRoundTrip asserts the two new keys survive the
// decode and re-encode the CLI puts every list config through.
func TestKarpenterNodePoolVolumeRoundTrip(t *testing.T) {
	in := `[{"name":"gpu","volume_iops":4000,"volume_throughput":600}]`
	params := map[string]string{"additional_karpenter_nodepools_config": in}
	if err := validateAndMutateParams(params, "aws", map[string]string{}, false); err != nil {
		t.Fatalf("validate: %v", err)
	}
	out, err := base64.StdEncoding.DecodeString(params["additional_karpenter_nodepools_config"])
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, want := range []string{`"volume_iops":4000`, `"volume_throughput":600`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("re-encoded config %s is missing %s", out, want)
		}
	}
}

func copyParams(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

// TestKarpenterNodeConfigUnchangedWithoutFastImagePull pins the bytes of the
// Karpenter EC2NodeClass userData, the only NodeConfig that reaches a pool.
// The heredoc holding it is flush-left because a %{ if } inside a <<- heredoc
// cancels the unindenting, and a re-indent would change every rendered line
// while terraform fmt, terraform validate and the rest of this suite stay green.
func TestKarpenterNodeConfigUnchangedWithoutFastImagePull(t *testing.T) {
	const path = "../../terraform/cluster/aws/karpenter_nodepool.tf"

	body := regexp.MustCompile(`(?s)ec2_node_config\s*=\s*<<EOT\n(.*?)\nEOT\n`).
		FindStringSubmatch(readTestFile(t, path))
	if body == nil {
		t.Fatalf("no flush-left ec2_node_config heredoc in %s", path)
	}

	expr, diags := hclsyntax.ParseTemplate([]byte(body[1]+"\n"), path, hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatalf("parse: %v", diags)
	}

	for _, tc := range []struct {
		name          string
		registrySet   bool
		fastImagePull bool
		want          string
	}{
		{"registry off-default", true, false, wantKarpenterNodeConfigRegistry},
		{"both", true, true, wantKarpenterNodeConfigBoth},
		{"gate only", false, true, wantKarpenterNodeConfigGate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, diags := expr.Value(&hcl.EvalContext{Variables: map[string]cty.Value{
				"var": cty.ObjectVal(map[string]cty.Value{
					"fast_image_pull_enable": cty.BoolVal(tc.fastImagePull),
				}),
				"local": cty.ObjectVal(map[string]cty.Value{
					"kubelet_registry_set":                cty.BoolVal(tc.registrySet),
					"kubelet_registry_pull_qps_effective": cty.NumberIntVal(20),
					"kubelet_registry_burst_effective":    cty.NumberIntVal(40),
				}),
			}})
			if diags.HasErrors() {
				t.Fatalf("render: %v", diags)
			}
			if got := v.AsString(); got != tc.want {
				t.Errorf("render changed\n--- want ---\n%q\n--- got ---\n%q", tc.want, got)
			}
		})
	}
}

const wantKarpenterNodeConfigRegistry = `apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  kubelet:
    config:
      registryPullQPS: 20
      registryBurst: 40
`

const wantKarpenterNodeConfigBoth = `apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  featureGates:
    FastImagePull: true
  kubelet:
    config:
      registryPullQPS: 20
      registryBurst: 40
`

const wantKarpenterNodeConfigGate = `apiVersion: node.eks.aws/v1alpha1
kind: NodeConfig
spec:
  featureGates:
    FastImagePull: true
`
