package k8s_test

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	metricfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func newTestProvider() (*k8s.Provider, *fake.Clientset) {
	c := fake.NewSimpleClientset()
	cc := cvfake.NewSimpleClientset()
	mc := metricfake.NewSimpleClientset()
	a := &atom.MockInterface{}

	p := &k8s.Provider{
		Atom:          a,
		Cluster:       c,
		Convox:        cc,
		Domain:        "domain1",
		Engine:        &mock.TestEngine{},
		MetricsClient: mc,
		Name:          "rack1",
		Namespace:     "ns1",
		Provider:      "test",
	}

	return p, c
}

func nodeCreator(c kubernetes.Interface, name string, fn func(n *ac.Node)) error {
	n := &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name: name,
		},
	}

	if fn != nil {
		fn(n)
	}

	if _, err := c.CoreV1().Nodes().Create(context.TODO(), n, am.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func nodeCreateResources(c kubernetes.Interface, name, cpu, mem string) error {
	return nodeCreator(c, name, func(n *ac.Node) {
		n.Status.Capacity = ac.ResourceList{
			ac.ResourceCPU:    resource.MustParse(cpu),
			ac.ResourceMemory: resource.MustParse(mem),
		}
	})
}

func TestInstanceShellError(t *testing.T) {
	a := &atom.MockInterface{}
	c := fake.NewSimpleClientset()
	cc := cvfake.NewSimpleClientset()
	mc := metricfake.NewSimpleClientset()

	_, err := c.CoreV1().Namespaces().Create(
		context.TODO(), &ac.Namespace{
			ObjectMeta: am.ObjectMeta{
				Name: "kube-system",
				Labels: map[string]string{
					"app":    "system",
					"rack":   "rack1",
					"system": "convox",
					"type":   "rack",
				},
				UID: "uid1",
			},
		},
		am.CreateOptions{},
	)
	require.NoError(t, err)

	_, err = c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name: "id1",
			Labels: map[string]string{
				"app":    "system",
				"rack":   "rack1",
				"system": "convox",
				"type":   "rack",
			},
			UID: "uid1",
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	p := &k8s.Provider{
		Atom:          a,
		Cluster:       c,
		Convox:        cc,
		Domain:        "domain1",
		Engine:        &mock.TestEngine{},
		MetricsClient: mc,
		Name:          "rack1",
		Namespace:     "ns1",
		Provider:      "test",
	}

	httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	code, err := p.InstanceShell("id1", bufio.NewReadWriter(bufio.NewReader(&bytes.Buffer{}), bufio.NewWriter(&bytes.Buffer{})), structs.InstanceShellOptions{
		PrivateKey: options.String("LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlKSndJQkFBS0NBZ0VBdHdOZnV3UDhaUXhzK1NOTnhhOVFXQU4xcXpUcmE2MjhvY0pYWEdqcGplKzJpNGZjCmVCSEFmQ2lrVExxV1kwN3p1NURmUloyRjhJQkhxWnZQNWZOQktsNHZwNlBEQTNqbWt2MjdKZ2IwOWVBaXdJV0oKMlREZGxjRm9pa2RHc2dYWXV1K0VGck80Z3piMysrbTJRcUh2MXI0SnFWOHNkdDN0dDJUVDZ3NFYyWFBiTXljdgpDVjVJODBkM3BWekI3RVBBVUM0SWpMUHE5VDBHRytGQisyUURZZEZCVFFkeVVvQ1dFTWxYRXN6cDRMK0FjV2E5CnRZclFZUk1zWm5QSVduMm1reUwxZi9ReUt6bnVPYVNtVDRVWG96ZTlpcTNJZ2xmS1ZleE1aN0cvMzZ1WkpoQjIKSmlKTTdtVGRVVGttQzFSR0FCU1FDd0M3ZzFadkpmZlhvV3NVOEFuWFRsRGU4bjBmSGU1ckJWeVlmL21BTGxTMQpMaitLSXhraU4vaHE4OHZmaUlSNFNzbHRQSUJvOHYyaDBxd2RRVE1vdzNOaDF3VWFickU2Tk1lOTRGTXo4eExDCkVCay83clluemU2VVRyc003ZlJDczlGaS9GMFRDRk5QdnZOWis2bW5md0toUEdWdmZvRC9CR1Y0VHU3bm1kR3YKTEFPQ2kxK01hQWZDcHJiTUd5c1hWZGwxVVEzT2wxeGpTTHRkY2xaWENzYnBpdWozUzlkS0ZWVFcyZFova2Q4NQpNQXRsMUc0VFBaMC93S3NCZnJObGFCT1FEOW5MdW1hbE51UWQ0T0NwMUFONHBrL0ZQVnMrZmhkQjNLeGdRTzRCCmxIcncvU2pKdm1oR2czeHRZaUEwSEJXTXRldlBvbE5ZVTlNVVMwc3RxeXpWRTRpTmFpSkFWZGRGaGNNQ0F3RUEKQVFLQ0FnQXVEMDFTbGdnNXJrem16dm5OM1BlTmp5RllPM05jb0Zjdkp1Z3h1NzI4R1M2S0kyRmJYcXhoRXlGMgpwaWNmUzBtUVZUKzhGNDhVSGxUcTNPb1A1NDdwQ05kWmk1K1RDaVNOcmdvaDRmSll4MVkzdWVRZG8yekJPZklECml6akJxVE1JcVN0SFEyZ1dyZ2p2Zjd4OVBLQk9IWG4xQkp6K05aQ1ViVzNnWktVZkcwZDVza3ptUUxKL2QrY20KMlJkOVRQZmp0aEkweHp0RkNWeTJPNVVObmZnejhDUk5MS2liRnYydHI5NllQclpGK0N4dFhmdzA2b0RUVGE0SgpBdTdUeDNmYTVCdUJYb1laMXZTYjBWS0NCTzhVQnYxUEg3bXRCRWRLSkxSK0RJQkwzTFlvbkUvLy9QWDdzYXI1CnZEWlU5NXErZm01YU5vNzYyUkFVTURJMnorcHVxZ0F1SGlabC9hY240SWp1ME1ISTk4d3RaalVZY3k4MG9FY0kKaHVyOHFpMDVmM3NtZlRSZi9HYjBTQ2FlK3Vtb05rQjdQUVU3bjdzWUw3aGErUFRWTU9yUWpjZlZVRDd6cU1MaQo2S2dMNnpTVGJEQlBEcm45eHYxUmNrU1lDeHcrc0hMcVBPVThldUx5MEpleEI3QmdXVkZXbXliMlpKY2g2eWlHCitoSlhNaG9FTkxZL2R0cURaSjZrdldqd1o1TC9raEYzOFZua1lwK2txZmd5d3hJaW1HLzVCSFBwdGpqT1pPemQKRFB6VnFSUmM1NFNieGg2Y0hJWTNML2E4OENxYkdXcGJ5MXR5SVNHemRDSlhITkhmTmJ6UmpNSXdpY3Rmc2NCKwptcEN3N1hTME9laVB3TTlMNTJoeUp4RVJZQ3NwSG9hVk5hNDNqcWxFeG5xc2VWc2VZUUtDQVFFQXl6eHJGdDQ0Cjl3UGR3VFdsczEzZUFmRkl1bStwQVk1YldSNnZSWDFlaCttTDBaMisvTEpPaldUUUg0NEw5MEJxc004enF0alQKc1IzVlp1QlJ0WEN4aVBidkxxcHZNeW0xSTRqYXNlbGJuMCtET1BQRUVoSlI5MnorSDU3T0Y4eHVFTnZtYlY4MwovSkl0cXBjdTIyOW8vYkdaQ2JGNUpQZFpUbUxWcWhmb2Y5djc1S1dVdElwb1UwdkxtdXBZRlNmWXVHWWxOZFNoCjRsSXpiMHBoZFRPQmVPWTgvVWRCVkhTZXNzUmVFZXFDUUNWUWlhTGNHU1RxcmU3OTN0NjhEblRMU1ROdWJ2ZmgKbVBSYTBPT0NuRGYvTGRXaG45MUpkeDdoaWFUSlF0TDlRd1lvTjFDUUFQenNXeFZtQkJPRTJkZDkzaGU1NVpONgp2L3VXUVFFUEt2QmppUUtDQVFFQTVvYmxIVy9DSHgrcWQ4SUFqZThrMEZXN0VkWUtPRTdwbjl3cEoxZHcvYWRGClMwSUNGLzRkdGRHUEQ3TGd1Z0Z6RlptemxQKzB2ekYyd25GUUNjU2c1eHRmMDdKQ3Y1ZFVEa0JIUmc3ODNmV2YKblZ0OU1CS3E4TXFCeDlJdWV0S25vVWFhYXR6OGpkLzliNWFHeTlvbUpCZk0vd2pSazQzN1BqcFZCTFpkSy9ZagpzK3ZHdHZBRzhnc3ozbENtbzF6aGhoVytNWG5raEZUMC9tbTZ0czN3ZmRlaitFL0V4bnhyL3gxTTJCSFB1WFQvCmtsREtWRmNPeGRMT3Y3NlV6V2tXaGNiMlJQU0h6VE1YYUNhWWlIYmFEZ1Bwci8wcEZqTGZXanFtY0gxZlU2dU4KdmxtQ3J6bkNrZTdEeUYvRzJqL3E2NldiYWNpTDR6aTNTMk1OWHJRdjZ3S0NBUUJPT3l5UnVlcTlrdHhxZ2djTgptMFZaQkJMVnlTT0tPTTBLNmhmWHJPR1ZlWjNiaTFnNEZ6N0xpSkhnZzZJeGc3ZE41Z1JpY0dKVVhFS0gvak1WCm41S0hRVjVpWFRLK3hBQTQ5SFlTWTl4ajM0eUlnTFRwcSttblQyb21xODl6TTdydWZHY2ZsTGFOWDR0NmpnVjQKYkZOQ3pIazNWUHhuOFZxTUpObFFSekcwL2UzaFhxZDJNNHppKzFzZGY3VjJOMGRoKzllNDkrZDBvZVd3b2pZUApra3IxU2RNU1A3cHpFd3ovalQwVXNtdCsyNTQ3ek5maVNlYVlHMVhYMDI4YU5YVUc4V2hDQ09McktLeEltanJ0CmZWU1p4UkVZNDJwakV4MElDY2w3RXBKd21VOGpzN3dxMVRENkFxdXBTQVlzZ0ptdnd2eEkxZE03VVZIT0plVTcKeHhyWkFvSUJBRDFMelF3Z3RVczFUa3g5cDB3V05JRzQ2Zjc2SE1tUUlnckJyYWVxVm43N3hTOFhwQnJoRk9GTgprVzZwTDV3RjJxWDVzb2lCM1h1aEdXVG1ZRlBZbHJnY3pVUXErQmlka2xvZnVWUGxsRk1vaG9KKzJ6QzNKZ0RVCnQrRjcvNlBrODNHc3BrNlg2QWVKY1lnRVRGOVQ2Q1JjdDhOd2d0bHJQVXowa2JxcEN1a3ZqNXE5cko3ai85VEkKS2NpSlJXWFd3cGkybTFYb2hWeXlnbHBMbnJ3UTBrdUQxVE9uY2prY3lGS1RaZVZtQndmWEZrVy9lYTBuWU9RMQpBNGgwa25oejR1czdWaXhMb1BodEIySklNUExJTjM3V2g0S21IQkN0QWpxcHNhdmppajlqTkZOUzFhN2wrdGgyCjFsUW5NMmJBN0NRd2FyeWIrS1k0WkhTRkovN3doblVDZ2dFQU5zQTlsWDRZZzJtbENqblFJRkNyNFU4RmtyeW0KQWZlbUZsaS9sYXZBNVh0MjVLRzZFb3YvRVl5WXNQSE5qUFRJMldiK2FJQXJ0NnRRNG9XSm1QbmI1Y05YUTRFaQoweWczV0d4U0w4WktieHBUcGVVNVJhek1IN3RXRGRtbVc0Q2cvaVpGd2Q0MElXQzd0MHFBWTExZGFpb3p3SkZkCksyam4vdmFtU2tpcjVsdVZadnNCaFJrd1AxTy80eG9HVEJ5dm9uaGdyREdHU0UxQ1lRR1ZLa1ZoYzNKU1RyQTMKVmhiVUJWSlYwZHRHcEVOSGgzY0lVY051aXFRSlFGc2dmNnRWU25DempYVHFFUDlvYVpGY0o4MGpHYkxoaWE2MwovbUFGVU5WVnplRDZHSWNaTG4wSVNoNlRYZzIvajJVclArbWk3aUNvUW5MTE5YN2h3MGNQYytxSUxnPT0KLS0tLS1FTkQgUlNBIFBSSVZBVEUgS0VZLS0tLS0K"),
	})
	require.Error(t, err)
	require.Equal(t, 0, code)
}

func TestInstanceTerminateNotFound(t *testing.T) {
	p, _ := newTestProvider()

	err := p.InstanceTerminate("nonexistent-node")
	require.Error(t, err)
	require.Contains(t, err.Error(), "instance not found")
}

func TestInstanceTerminateReadyNode(t *testing.T) {
	p, c := newTestProvider()

	// create a Ready node
	_, err := c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{Name: "node-ready"},
		Status: ac.NodeStatus{
			Conditions: []ac.NodeCondition{
				{Type: ac.NodeReady, Status: ac.ConditionTrue},
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	// create a regular pod on the node
	_, err = c.CoreV1().Pods("default").Create(context.TODO(), &ac.Pod{
		ObjectMeta: am.ObjectMeta{Name: "app-pod", Namespace: "default"},
		Spec:       ac.PodSpec{NodeName: "node-ready"},
	}, am.CreateOptions{})
	require.NoError(t, err)

	// create a DaemonSet pod on the node — should be skipped
	_, err = c.CoreV1().Pods("kube-system").Create(context.TODO(), &ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      "ds-pod",
			Namespace: "kube-system",
			OwnerReferences: []am.OwnerReference{
				{Kind: "DaemonSet", Name: "fluentd", APIVersion: "apps/v1"},
			},
		},
		Spec: ac.PodSpec{NodeName: "node-ready"},
	}, am.CreateOptions{})
	require.NoError(t, err)

	err = p.InstanceTerminate("node-ready")
	require.NoError(t, err)

	// node should be deleted
	_, err = c.CoreV1().Nodes().Get(context.TODO(), "node-ready", am.GetOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")

	// DaemonSet pod should still exist (was skipped)
	_, err = c.CoreV1().Pods("kube-system").Get(context.TODO(), "ds-pod", am.GetOptions{})
	require.NoError(t, err)
}

func TestInstanceTerminateNotReadyNode(t *testing.T) {
	p, c := newTestProvider()

	// create a NotReady node
	_, err := c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{Name: "node-notready"},
		Status: ac.NodeStatus{
			Conditions: []ac.NodeCondition{
				{Type: ac.NodeReady, Status: ac.ConditionFalse},
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	// create stuck pods on the NotReady node
	for _, name := range []string{"stuck-pod-1", "stuck-pod-2"} {
		_, err = c.CoreV1().Pods("app-ns").Create(context.TODO(), &ac.Pod{
			ObjectMeta: am.ObjectMeta{Name: name, Namespace: "app-ns"},
			Spec:       ac.PodSpec{NodeName: "node-notready"},
		}, am.CreateOptions{})
		require.NoError(t, err)
	}

	err = p.InstanceTerminate("node-notready")
	require.NoError(t, err)

	// node should be deleted
	_, err = c.CoreV1().Nodes().Get(context.TODO(), "node-notready", am.GetOptions{})
	require.Error(t, err)

	// pods should be force-deleted
	pods, err := c.CoreV1().Pods("app-ns").List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, pods.Items)
}

func TestInstanceTerminateAlreadyCordoned(t *testing.T) {
	p, c := newTestProvider()

	// create an already-cordoned node
	_, err := c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{Name: "node-cordoned"},
		Spec:       ac.NodeSpec{Unschedulable: true},
		Status: ac.NodeStatus{
			Conditions: []ac.NodeCondition{
				{Type: ac.NodeReady, Status: ac.ConditionTrue},
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	err = p.InstanceTerminate("node-cordoned")
	require.NoError(t, err)

	// node should be deleted
	_, err = c.CoreV1().Nodes().Get(context.TODO(), "node-cordoned", am.GetOptions{})
	require.Error(t, err)
}

func TestInstanceTerminateMirrorPodSkipped(t *testing.T) {
	p, c := newTestProvider()

	_, err := c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{Name: "node-mirror"},
		Status: ac.NodeStatus{
			Conditions: []ac.NodeCondition{
				{Type: ac.NodeReady, Status: ac.ConditionTrue},
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	// create a mirror pod (managed by kubelet)
	_, err = c.CoreV1().Pods("kube-system").Create(context.TODO(), &ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      "kube-apiserver-node-mirror",
			Namespace: "kube-system",
			Annotations: map[string]string{
				ac.MirrorPodAnnotationKey: "mirror-hash",
			},
		},
		Spec: ac.PodSpec{NodeName: "node-mirror"},
	}, am.CreateOptions{})
	require.NoError(t, err)

	err = p.InstanceTerminate("node-mirror")
	require.NoError(t, err)

	// mirror pod should still exist (was skipped)
	_, err = c.CoreV1().Pods("kube-system").Get(context.TODO(), "kube-apiserver-node-mirror", am.GetOptions{})
	require.NoError(t, err)
}
