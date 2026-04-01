package provider_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type ParamsFile struct {
	Groups []struct {
		Name   string `yaml:"name"`
		Order  int32  `yaml:"order"`
		Params []struct {
			Name            string   `yaml:"name"`
			Default         string   `yaml:"default"`
			Type            string   `yaml:"type"`
			Regex           string   `yaml:"regex,omitempty"`
			SideNote        string   `yaml:"sideNote,omitempty"`
			Description     string   `yaml:"description"`
			Optional        bool     `yaml:"optional,omitempty"`
			AllowedValues   []string `yaml:"allowedValues,omitempty"`
			AllowedMinValue int32    `yaml:"allowedMinValue,omitempty"`
			AllowedMaxValue int32    `yaml:"allowedMaxValue,omitempty"`
		} `yaml:"params"`
	} `yaml:"Groups"`
}

var providers = []string{"aws", "gcp", "azure", "do"}

func loadParams(t *testing.T, provider string) ParamsFile {
	t.Helper()
	path := filepath.Join(provider, "params.yaml")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read %s", path)
	var params ParamsFile
	require.NoError(t, yaml.Unmarshal(data, &params), "failed to parse %s", path)
	return params
}

func TestParamsYAML_StructureValid(t *testing.T) {
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			params := loadParams(t, provider)
			require.NotEmpty(t, params.Groups, "should have at least one group")

			allNames := map[string]bool{}
			for _, g := range params.Groups {
				assert.NotEmpty(t, g.Name, "group name required")
				assert.Greater(t, g.Order, int32(0), "group order must be positive")
				require.NotEmpty(t, g.Params, "group %q should have params", g.Name)

				for _, p := range g.Params {
					t.Run(p.Name, func(t *testing.T) {
						// No duplicate param names
						assert.False(t, allNames[p.Name], "duplicate param name %q", p.Name)
						allNames[p.Name] = true

						// Required fields
						assert.NotEmpty(t, p.Name)
						assert.NotEmpty(t, p.Type)
						assert.NotEmpty(t, p.Description, "param %q missing description", p.Name)

						// Valid type
						assert.Contains(t, []string{"boolean", "string", "integer"}, p.Type,
							"param %q has invalid type %q", p.Name, p.Type)
					})
				}
			}
		})
	}
}

func TestParamsYAML_DefaultsMatchType(t *testing.T) {
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			params := loadParams(t, provider)
			for _, g := range params.Groups {
				for _, p := range g.Params {
					t.Run(p.Name, func(t *testing.T) {
						if p.Default == "null" || p.Default == "" {
							return // null/empty defaults are valid for optional params
						}
						switch p.Type {
						case "boolean":
							assert.Contains(t, []string{"true", "false"}, p.Default,
								"boolean param %q default %q invalid", p.Name, p.Default)
						case "integer":
							for _, c := range p.Default {
								if c != '-' && (c < '0' || c > '9') {
									t.Errorf("integer param %q default %q not numeric", p.Name, p.Default)
									break
								}
							}
						}
					})
				}
			}
		})
	}
}

func TestParamsYAML_RegexCompiles(t *testing.T) {
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			params := loadParams(t, provider)
			for _, g := range params.Groups {
				for _, p := range g.Params {
					if p.Regex == "" {
						continue
					}
					t.Run(p.Name, func(t *testing.T) {
						_, err := regexp.Compile(p.Regex)
						require.NoError(t, err, "regex for %q does not compile: %s", p.Name, p.Regex)
					})
				}
			}
		})
	}
}

func TestParamsYAML_DefaultMatchesRegex(t *testing.T) {
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			params := loadParams(t, provider)
			for _, g := range params.Groups {
				for _, p := range g.Params {
					if p.Regex == "" || p.Default == "null" || p.Default == "" {
						continue
					}
					t.Run(p.Name, func(t *testing.T) {
						matched, err := regexp.MatchString(p.Regex, p.Default)
						require.NoError(t, err)
						assert.True(t, matched,
							"default %q does not match regex %q", p.Default, p.Regex)
					})
				}
			}
		})
	}
}

func TestParamsYAML_RegexAcceptsValidInputs(t *testing.T) {
	// Table-driven regex validation with known-good and known-bad inputs
	testCases := map[string][]struct {
		input string
		valid bool
	}{
		// AWS instance types
		"aws/node_type": {
			{"t3.small", true},
			{"m5.large", true},
			{"c7gn.xlarge", true},
			{"m5.metal", true},
			{"u-6tb1.metal", true},
			{"dl1.24xlarge", true},
			{"t3.small,m5.large", true},
			{"INVALID", false},
			{"", false},
		},
		"aws/build_node_type": {
			{"t3.large", true},
			{"m6a.large", true},
			{"t3.small,m5.large", false}, // no comma-sep for build
		},
		// AWS AZs
		"aws/availability_zones": {
			{"us-east-1a", true},
			{"us-east-1a,us-east-1b,us-east-1c", true},
			{"eu-central-1a", true},
			{"us-east-1", false},
		},
		// AWS resources
		"aws/vpc_id":                {{"vpc-abc123", true}, {"sg-123", false}},
		"aws/internet_gateway_id":   {{"igw-abc123", true}, {"vpc-123", false}},
		"aws/nlb_security_group":    {{"sg-abc123", true}, {"vpc-123", false}},
		"aws/private_subnets_ids":   {{"subnet-abc", true}, {"subnet-a,subnet-b", true}},
		"aws/key_pair_name":         {{"my-key", true}, {"my key", false}},
		"aws/user_data_url":         {{"https://example.com/s.sh", true}, {"ftp://x.com", false}},
		// CIDR
		"aws/cidr": {
			{"10.0.0.0/16", true},
			{"192.168.1.0/24", true},
			{"256.0.0.0/16", false},
		},
		// Tags (AWS and Azure share format)
		"aws/tags": {
			{"k=v", true},
			{"k1=v1,k2=v2", true},
			{"k", false},
		},
		// SSL
		"aws/ssl_protocols": {
			{"TLSv1.2", true},
			{"TLSv1.2 TLSv1.3", true},
			{"TLSv1.4", false},
		},
		"aws/ssl_ciphers": {
			{"ECDHE-RSA-AES128-GCM-SHA256", true},
			{"A:B:C", true},
		},
		// Syslog (all providers share)
		"aws/syslog": {
			{"tcp://host:514", true},
			{"tcp+tls://logs.example.com:5514", true},
			{"tcp://host_under.com:514", true},
			{"http://x.com:80", false},
		},
		// Cert duration (all providers share)
		"aws/cert_duration": {
			{"2160h", true},
			{"720h", true},
			{"90d", false},
		},
		// GCP
		"gcp/node_type": {
			{"n1-standard-1", true},
			{"e2-small", true},
			{"n2d-highcpu-16", true},
			{"a2-highgpu-1g", true},
		},
		"gcp/region": {
			{"us-east1", true},
			{"europe-west1", true},
			{"us-east-1", false},
		},
		// Azure
		"azure/node_type": {
			{"Standard_D2_v3", true},
			{"Standard_B2s", true},
			{"Standard_NC6s_v3", true},
			{"D2_v3", false},
		},
		"azure/region": {
			{"eastus", true},
			{"westus2", true},
			{"east-us", false},
		},
		// DO
		"do/node_type": {
			{"s-2vcpu-4gb", true},
			{"g-2vcpu-8gb", true},
			{"so1_5-2vcpu-16gb", true},
		},
		"do/region": {
			{"nyc3", true},
			{"sfo3", true},
			{"nyc", false},
		},
		"do/registry_disk": {
			{"50Gi", true},
			{"100Gi", true},
			{"50GB", false},
		},
	}

	// Build regex lookup from all providers
	regexMap := map[string]string{}
	for _, provider := range providers {
		params := loadParams(t, provider)
		for _, g := range params.Groups {
			for _, p := range g.Params {
				if p.Regex != "" {
					regexMap[fmt.Sprintf("%s/%s", provider, p.Name)] = p.Regex
				}
			}
		}
	}

	for paramKey, cases := range testCases {
		t.Run(paramKey, func(t *testing.T) {
			pattern, ok := regexMap[paramKey]
			require.True(t, ok, "no regex found for %s", paramKey)
			re := regexp.MustCompile(pattern)

			for _, tc := range cases {
				t.Run(tc.input, func(t *testing.T) {
					matched := re.MatchString(tc.input)
					if tc.valid {
						assert.True(t, matched,
							"%q should match regex %q", tc.input, pattern)
					} else {
						assert.False(t, matched,
							"%q should NOT match regex %q", tc.input, pattern)
					}
				})
			}
		})
	}
}

func TestParamsYAML_GroupOrderConsistent(t *testing.T) {
	// All providers should use consistent group order numbers
	expectedOrders := map[string]int32{
		"Immutable":            1,
		"Security & Compliance": 2,
		"Networking":           3,
		"Performance & Scaling": 4,
		"Logging & Monitoring":  5,
	}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			params := loadParams(t, provider)
			for _, g := range params.Groups {
				if expected, ok := expectedOrders[g.Name]; ok {
					assert.Equal(t, expected, g.Order,
						"group %q in %s has order %d, expected %d", g.Name, provider, g.Order, expected)
				} else {
					t.Errorf("unexpected group name %q in %s", g.Name, provider)
				}
			}
		})
	}
}

func TestParamsYAML_SharedParamsConsistent(t *testing.T) {
	// Params that appear in multiple providers should have consistent types
	type paramInfo struct {
		provider string
		ptype    string
	}

	allParams := map[string][]paramInfo{}
	for _, provider := range providers {
		params := loadParams(t, provider)
		for _, g := range params.Groups {
			for _, p := range g.Params {
				allParams[p.Name] = append(allParams[p.Name], paramInfo{provider, p.Type})
			}
		}
	}

	for name, infos := range allParams {
		if len(infos) <= 1 {
			continue
		}
		t.Run(name, func(t *testing.T) {
			baseType := infos[0].ptype
			for _, info := range infos[1:] {
				assert.Equal(t, baseType, info.ptype,
					"param %q type mismatch: %s has %q, %s has %q",
					name, infos[0].provider, baseType, info.provider, info.ptype)
			}
		})
	}
}
