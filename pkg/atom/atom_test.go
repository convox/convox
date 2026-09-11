package atom

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	aa "github.com/convox/convox/pkg/atom/pkg/apis/atom/v1"
	av "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned"
	afake "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestStatus(t *testing.T) {
	tests := []struct {
		Name          string
		AtomNamespace string
		AtomName      string
		AtomStatus    string
		AtomRelease   string
		AtomVersion   string
		AtomSpec      aa.AtomSpec
	}{
		{
			Name:          "Success",
			AtomNamespace: "ns1",
			AtomName:      "atom1",
			AtomStatus:    "Updating",
			AtomRelease:   "",
			AtomSpec:      aa.AtomSpec{},
			AtomVersion:   "v1",
		},
		{
			Name:          "With Current Version",
			AtomNamespace: "ns2",
			AtomName:      "atom2",
			AtomStatus:    "Updating",
			AtomRelease:   "v1.0.0",
			AtomSpec: aa.AtomSpec{
				CurrentVersion: "v1.0.0",
			},
			AtomVersion: "v2",
		},
	}

	testClient(t, func(ac *Client) {
		fac := ac.Atom.(*afake.Clientset)

		for _, test := range tests {
			fn := func(t *testing.T) {
				version := test.AtomVersion
				if test.AtomSpec.CurrentVersion != "" {
					version = test.AtomSpec.CurrentVersion
				}

				require.NoError(t, atomCreate(
					fac,
					test.AtomNamespace,
					test.AtomName,
					test.AtomStatus,
					version,
					test.AtomSpec,
				))

				st, release, err := ac.Status(test.AtomNamespace, test.AtomName)
				assert.Equal(t, test.AtomStatus, st)
				assert.Equal(t, test.AtomRelease, release)
				require.NoError(t, err)
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestCancel(t *testing.T) {
	testClient(t, func(ac *Client) {
		fac := ac.Atom.(*afake.Clientset)

		require.NoError(t, atomCreate(fac, "ns1", "atom1", "Updating", "atom1", aa.AtomSpec{}))
		require.NoError(t, atomCreate(fac, "ns1", "atom2", "Rollback", "atom2", aa.AtomSpec{}))
		require.NoError(t, atomCreate(fac, "ns1", "atom3", "Other", "atom3", aa.AtomSpec{}))

		require.NoError(t, ac.Cancel("ns1", "atom1"))
		a, err := fac.AtomV1().Atoms("ns1").Get(context.Background(), "atom1", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, aa.AtomStatus("Cancelled"), a.Status)

		require.NoError(t, ac.Cancel("ns1", "atom2"))
		a, err = fac.AtomV1().Atoms("ns1").Get(context.Background(), "atom2", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, aa.AtomStatus("Failure"), a.Status)

		err = ac.Cancel("ns1", "atom3")
		require.EqualError(t, err, "not currently updating")
	})
}

func TestApply(t *testing.T) {
	tests := []struct {
		Name          string
		AtomNamespace string
		AtomName      string
		AtomRelease   string
	}{
		{
			Name:          "Success",
			AtomNamespace: "ns1",
			AtomName:      "atom1",
			AtomRelease:   "1.0",
		},
	}

	testClient(t, func(ac *Client) {
		fac := ac.Atom.(*afake.Clientset)

		for _, test := range tests {
			fn := func(t *testing.T) {
				require.NoError(t, ac.Apply(test.AtomNamespace, test.AtomName, &ApplyConfig{
					Release:  test.AtomRelease,
					Template: nil,
					Timeout:  600,
				}))

				a, err := fac.AtomV1().Atoms(test.AtomNamespace).Get(context.Background(), test.AtomName, am.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, aa.AtomStatus("Pending"), a.Status)
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestIsRecoverableApplyError(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{
			name: "duplicate service port strategic-merge delete (k8s #105610)",
			out: `Error from server (Invalid): error when applying patch:` +
				`{"spec":{"$setElementOrder/ports":[{"port":8080}],"ports":[{"$patch":"delete","name":"main","port":8080}]}}` +
				` to:` + "\n" +
				`for: "STDIN": error when patching "STDIN": Service "web" is invalid: spec.ports: Required value`,
			want: true,
		},
		{
			name: "immutable field",
			out:  `The Service "web" is invalid: spec.clusterIP: Invalid value: "": field is immutable`,
			want: true,
		},
		{
			name: "genuinely invalid object must NOT force-recreate",
			out:  `The Service "web" is invalid: spec.ports[0].port: Invalid value: 99999999: must be between 1 and 65535`,
			want: false,
		},
		{
			name: "patch failure without delete directive must NOT force-recreate",
			out: `Error from server: error when applying patch:` +
				`{"spec":{"$setElementOrder/ports":[{"port":8080}]}}` +
				` to:` + "\n" +
				`for: "STDIN": error when patching "STDIN": Service "web" is invalid: spec.ports: Required value`,
			want: false,
		},
		{
			name: "clean apply output",
			out:  "service/web configured\n",
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRecoverableApplyError([]byte(tc.out)); got != tc.want {
				t.Fatalf("isRecoverableApplyError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStatefulSetReady(t *testing.T) {
	tests := []struct {
		name       string
		desired    int
		generation int
		observed   int
		replicas   int
		ready      int
		available  int
		updated    int
		current    string
		update     string
		want       bool
	}{
		{name: "ready", desired: 3, generation: 2, observed: 2, replicas: 3, ready: 3, available: 3, updated: 3, current: "rev2", update: "rev2", want: true},
		{name: "pvc or pod pending", desired: 3, generation: 2, observed: 2, replicas: 2, ready: 1, available: 1, updated: 2, current: "rev1", update: "rev2"},
		{name: "controller has not observed update", desired: 3, generation: 3, observed: 2, replicas: 3, ready: 3, available: 3, updated: 3, current: "rev3", update: "rev3"},
		{name: "rolling update incomplete", desired: 3, generation: 3, observed: 3, replicas: 3, ready: 3, available: 3, updated: 2, current: "rev2", update: "rev3"},
		{name: "scaled to zero", desired: 0, generation: 4, observed: 4, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{"metadata":{"generation":%d},"spec":{"replicas":%d},"status":{"observedGeneration":%d,"replicas":%d,"readyReplicas":%d,"availableReplicas":%d,"updatedReplicas":%d,"currentRevision":%q,"updateRevision":%q}}`,
				tt.generation, tt.desired, tt.observed, tt.replicas, tt.ready, tt.available, tt.updated, tt.current, tt.update))
			got, err := statefulSetReady(data)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestExtractStatefulSetConditions(t *testing.T) {
	data := []byte(`apiVersion: apps/v1
kind: StatefulSet
metadata:
  namespace: app
  name: database
  annotations:
    atom.conditions: Ready=True
`)
	conditions, err := extractConditions(data)
	require.NoError(t, err)
	require.Len(t, conditions, 1)
	require.Equal(t, "StatefulSet", conditions[0].Kind)
	require.Equal(t, "database", conditions[0].Name)
}

func atomCreate(ac av.Interface, namespace, name, status, version string, spec aa.AtomSpec) error {
	_, err := ac.AtomV1().Atoms(namespace).Create(context.Background(), &aa.Atom{
		ObjectMeta: am.ObjectMeta{
			Name: name,
		},
		Status: aa.AtomStatus(status),
		Spec:   spec,
	}, am.CreateOptions{})
	if err != nil {
		return err
	}

	_, err = ac.AtomV1().AtomVersions(namespace).Create(context.Background(), &aa.AtomVersion{
		ObjectMeta: am.ObjectMeta{
			Name: version,
		},
		Spec: aa.AtomVersionSpec{
			Release: version,
		},
	}, am.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func testClient(t *testing.T, fn func(*Client)) {
	fa := afake.NewSimpleClientset()
	c := fake.NewSimpleClientset()

	a := &Client{
		Atom: fa,
		k8s:  c,
	}

	fn(a)
}

type kubectlCall struct {
	data []byte
	args []string
}

func stubKubectl(t *testing.T, resources []string, fn func(call int, data []byte, args ...string) ([]byte, error)) *[]kubectlCall {
	t.Helper()

	calls := []kubectlCall{}

	ka := kubectlApply
	tr := templateResources

	kubectlApply = func(data []byte, args ...string) ([]byte, error) {
		i := len(calls)
		calls = append(calls, kubectlCall{data: data, args: args})

		if fn == nil {
			return []byte("applied\n"), nil
		}

		return fn(i, data, args...)
	}

	templateResources = func(_ string) ([]string, error) {
		return resources, nil
	}

	t.Cleanup(func() {
		kubectlApply = ka
		templateResources = tr
	})

	return &calls
}

const testDeploymentDocument = `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: ns1
  name: web
spec:
  replicas: 1
`

func testBalancerDocument(ports string) string {
	return `apiVersion: v1
kind: Service
metadata:
  namespace: ns1
  name: balancer-web
  labels:
    type: balancer
spec:
  type: LoadBalancer
` + ports
}

const testPairPorts = `  ports:
  - name: "5000-tcp"
    port: 5000
    protocol: TCP
    targetPort: 5000
  - name: "5000-udp"
    port: 5000
    protocol: UDP
    targetPort: 5000
`

const testSinglePorts = `  ports:
  - name: "5000"
    port: 5000
    protocol: TCP
    targetPort: 5000
`

func testStream(documents ...string) []byte {
	return []byte(strings.Join(documents, "---\n"))
}

func testLabelledStream(t *testing.T, data []byte, labels map[string]string) []byte {
	t.Helper()

	parts := bytes.Split(data, []byte("---\n"))

	for i := range parts {
		dp, err := applyLabels(parts[i], labels)
		require.NoError(t, err)

		parts[i] = dp
	}

	return bytes.Join(parts, []byte("---\n"))
}

func testService(ports []ac.ServicePort, managers ...am.ManagedFieldsEntry) *ac.Service {
	return &ac.Service{
		ObjectMeta: am.ObjectMeta{
			Namespace:     "ns1",
			Name:          "balancer-web",
			ManagedFields: managers,
		},
		Spec: ac.ServiceSpec{Ports: ports},
	}
}

func servicePort(name string, port int32, protocol ac.Protocol) ac.ServicePort {
	return ac.ServicePort{Name: name, Port: port, Protocol: protocol}
}

func TestDocumentPorts(t *testing.T) {
	cases := []struct {
		name           string
		ports          string
		paired         bool
		clientSideOnly bool
	}{
		{
			name:  "single port",
			ports: testSinglePorts,
		},
		{
			name:   "tcp and udp on one number",
			ports:  testPairPorts,
			paired: true,
		},
		{
			name: "two distinct ports",
			ports: `  ports:
  - name: "5000"
    port: 5000
    protocol: TCP
  - name: "5001"
    port: 5001
    protocol: TCP
`,
		},
		{
			name: "duplicate port name",
			ports: `  ports:
  - name: "5000"
    port: 5000
    protocol: TCP
  - name: "5000"
    port: 5000
    protocol: UDP
`,
			clientSideOnly: true,
		},
		{
			name: "duplicate port and protocol",
			ports: `  ports:
  - name: "5000-a"
    port: 5000
    protocol: TCP
  - name: "5000-b"
    port: 5000
    protocol: TCP
`,
			clientSideOnly: true,
		},
		{
			name: "unset protocol",
			ports: `  ports:
  - name: "5000"
    port: 5000
`,
			clientSideOnly: true,
		},
		{
			name: "unset protocol alongside a pair",
			ports: `  ports:
  - name: "5000-tcp"
    port: 5000
    protocol: TCP
  - name: "5000-udp"
    port: 5000
    protocol: UDP
  - name: "7000"
    port: 7000
`,
			clientSideOnly: true,
		},
		{
			name:  "no ports",
			ports: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			paired, clientSideOnly, err := documentPorts([]byte(testBalancerDocument(c.ports)))
			require.NoError(t, err)
			assert.Equal(t, c.paired, paired)
			assert.Equal(t, c.clientSideOnly, clientSideOnly)
		})
	}
}

func TestBalancerNeedsServerSide(t *testing.T) {
	kubectlManager := am.ManagedFieldsEntry{Manager: "kubectl", Operation: am.ManagedFieldsOperationApply}

	cases := []struct {
		name     string
		document string
		live     *ac.Service
		want     bool
	}{
		{
			name:     "empty document",
			document: "",
		},
		{
			name:     "deployment",
			document: testDeploymentDocument,
		},
		{
			name: "ingress carrying the balancer label alongside a live pair",
			document: `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  namespace: ns1
  name: balancer-web
  labels:
    type: balancer
`,
			live: testService([]ac.ServicePort{
				servicePort("5000-tcp", 5000, ac.ProtocolTCP),
				servicePort("5000-udp", 5000, ac.ProtocolUDP),
			}),
		},
		{
			name: "service without the balancer label",
			document: `apiVersion: v1
kind: Service
metadata:
  namespace: ns1
  name: balancer-web
spec:
` + testPairPorts,
			live: testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}),
		},
		{
			name:     "pair with no live service",
			document: testBalancerDocument(testPairPorts),
		},
		{
			name:     "pair over a live service",
			document: testBalancerDocument(testPairPorts),
			live:     testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}),
			want:     true,
		},
		{
			name:     "single port over an untouched live service",
			document: testBalancerDocument(testSinglePorts),
			live:     testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}),
		},
		{
			name:     "single port over a live pair",
			document: testBalancerDocument(testSinglePorts),
			live: testService([]ac.ServicePort{
				servicePort("5000-tcp", 5000, ac.ProtocolTCP),
				servicePort("5000-udp", 5000, ac.ProtocolUDP),
			}),
			want: true,
		},
		{
			name:     "a server-side field manager on its own does not qualify a single port",
			document: testBalancerDocument(testSinglePorts),
			live:     testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}, kubectlManager),
		},
		{
			name: "duplicate port names over a live service",
			document: testBalancerDocument(`  ports:
  - name: "5000"
    port: 5000
    protocol: TCP
  - name: "5000"
    port: 5000
    protocol: UDP
`),
			live: testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}),
		},
		{
			name: "unset protocol over a live pair",
			want: true,
			document: testBalancerDocument(`  ports:
  - name: "5000-tcp"
    port: 5000
    protocol: TCP
  - name: "5000-udp"
    port: 5000
    protocol: UDP
  - name: "7000"
    port: 7000
`),
			live: testService([]ac.ServicePort{
				servicePort("5000-tcp", 5000, ac.ProtocolTCP),
				servicePort("5000-udp", 5000, ac.ProtocolUDP),
			}, kubectlManager),
		},
		{
			name: "duplicate port names over a live pair",
			want: true,
			document: testBalancerDocument(`  ports:
  - name: "5000"
    port: 5000
    protocol: TCP
  - name: "5000"
    port: 5000
    protocol: UDP
`),
			live: testService([]ac.ServicePort{
				servicePort("5000-tcp", 5000, ac.ProtocolTCP),
				servicePort("5000-udp", 5000, ac.ProtocolUDP),
			}, kubectlManager),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			objects := []runtime.Object{}
			if c.live != nil {
				objects = append(objects, c.live)
			}

			client := &Client{k8s: fake.NewSimpleClientset(objects...)}

			got, err := client.balancerNeedsServerSide("ns1", []byte(c.document))
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestApplyTemplateWithoutBalancer(t *testing.T) {
	calls := stubKubectl(t, []string{"core/v1/Service", "apps/v1/Deployment"}, nil)

	data := testStream(testDeploymentDocument)

	c := &Client{k8s: fake.NewSimpleClientset()}

	_, err := c.applyTemplate("ns1", data, "atom=abc")
	require.NoError(t, err)

	require.Len(t, *calls, 1)
	assert.Equal(t, []string{
		"--prune", "-l", "atom=abc", "--namespace", "ns1",
		"--prune-allowlist", "core/v1/Service",
		"--prune-allowlist", "apps/v1/Deployment",
	}, (*calls)[0].args)
	assert.Equal(t, testLabelledStream(t, data, map[string]string{"atom": "abc"}), (*calls)[0].data)
}

func TestApplyTemplateBalancerWithoutLiveService(t *testing.T) {
	calls := stubKubectl(t, nil, nil)

	data := testStream(testDeploymentDocument, testBalancerDocument(testPairPorts))

	c := &Client{k8s: fake.NewSimpleClientset()}

	_, err := c.applyTemplate("ns1", data, "atom=abc")
	require.NoError(t, err)

	require.Len(t, *calls, 1)
	assert.Equal(t, []string{"--prune", "-l", "atom=abc", "--namespace", "ns1"}, (*calls)[0].args)
	assert.Equal(t, testLabelledStream(t, data, map[string]string{"atom": "abc"}), (*calls)[0].data)
}

func TestApplyTemplateForceRetrySendsTheWholeStream(t *testing.T) {
	calls := stubKubectl(t, nil, func(i int, _ []byte, _ ...string) ([]byte, error) {
		if i == 1 {
			return []byte(`The Service "web" is invalid: spec.clusterIP: field is immutable`), fmt.Errorf("exit status 1")
		}

		return []byte("applied\n"), nil
	})

	data := testStream(testDeploymentDocument, testBalancerDocument(testPairPorts))
	labelled := testLabelledStream(t, data, map[string]string{"atom": "abc"})

	c := &Client{k8s: fake.NewSimpleClientset(testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}))}

	_, err := c.applyTemplate("ns1", data, "atom=abc")
	require.NoError(t, err)

	require.Len(t, *calls, 3)
	assert.Equal(t, []string{"--force"}, (*calls)[2].args)
	assert.Equal(t, labelled, (*calls)[2].data)
}

func TestApplyTemplateServerSideLeg(t *testing.T) {
	calls := stubKubectl(t, nil, nil)

	data := testStream(testDeploymentDocument, testBalancerDocument(testPairPorts))
	labelled := testLabelledStream(t, data, map[string]string{"atom": "abc"})

	c := &Client{k8s: fake.NewSimpleClientset(testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}))}

	_, err := c.applyTemplate("ns1", data, "atom=abc")
	require.NoError(t, err)

	require.Len(t, *calls, 2)

	assert.Equal(t, []string{"--server-side", "--force-conflicts", "--namespace", "ns1"}, (*calls)[0].args)
	assert.NotContains(t, (*calls)[0].args, "--prune")

	balancer := bytes.Split(labelled, []byte("---\n"))[1]
	assert.Equal(t, balancer, (*calls)[0].data)

	assert.Equal(t, []string{"--prune", "-l", "atom=abc", "--namespace", "ns1"}, (*calls)[1].args)
	assert.Equal(t, labelled, (*calls)[1].data)
}

func TestApplyTemplateServerSideLegJoinsSeveralBalancers(t *testing.T) {
	calls := stubKubectl(t, nil, nil)

	second := strings.Replace(testBalancerDocument(testPairPorts), "balancer-web", "balancer-api", 1)

	data := testStream(testDeploymentDocument, testBalancerDocument(testPairPorts), second)
	labelled := testLabelledStream(t, data, map[string]string{"atom": "abc"})
	parts := bytes.Split(labelled, []byte("---\n"))

	api := testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)})
	api.Name = "balancer-api"

	c := &Client{k8s: fake.NewSimpleClientset(testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}), api)}

	_, err := c.applyTemplate("ns1", data, "atom=abc")
	require.NoError(t, err)

	require.Len(t, *calls, 2)
	assert.Equal(t, bytes.Join([][]byte{parts[1], parts[2]}, []byte("---\n")), (*calls)[0].data)
	assert.Equal(t, labelled, (*calls)[1].data)
}

func TestApplyTemplateServerSideLegFailureStopsTheApply(t *testing.T) {
	calls := stubKubectl(t, nil, func(_ int, _ []byte, _ ...string) ([]byte, error) {
		return []byte("server-side boom\n"), fmt.Errorf("exit status 1")
	})

	data := testStream(testBalancerDocument(testPairPorts))

	c := &Client{k8s: fake.NewSimpleClientset(testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}))}

	out, err := c.applyTemplate("ns1", data, "atom=abc")
	require.Error(t, err)
	assert.Equal(t, []byte("server-side boom\n"), out)
	assert.Len(t, *calls, 1)
}

func TestApplyTemplateServerSideOutputCannotTriggerForce(t *testing.T) {
	calls := stubKubectl(t, nil, func(i int, _ []byte, _ ...string) ([]byte, error) {
		if i == 0 {
			return []byte(`The Service "balancer-web" is invalid: spec.clusterIP: field is immutable`), nil
		}

		return []byte("unrelated failure\n"), fmt.Errorf("exit status 1")
	})

	data := testStream(testBalancerDocument(testPairPorts))

	c := &Client{k8s: fake.NewSimpleClientset(testService([]ac.ServicePort{servicePort("5000", 5000, ac.ProtocolTCP)}))}

	_, err := c.applyTemplate("ns1", data, "atom=abc")
	require.Error(t, err)
	assert.Len(t, *calls, 2)
}

func TestApplyTemplateServiceLookupFailureStopsTheApply(t *testing.T) {
	calls := stubKubectl(t, nil, nil)

	kc := fake.NewSimpleClientset()
	kc.PrependReactor("get", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("etcdserver: request timed out")
	})

	c := &Client{k8s: kc}

	_, err := c.applyTemplate("ns1", testStream(testBalancerDocument(testPairPorts)), "atom=abc")
	require.Error(t, err)
	assert.Empty(t, *calls)
}
