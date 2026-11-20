package k8s_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const hairpinRouterIP = "172.20.159.27"

func createHairpinService(t *testing.T, p *k8s.Provider, name, clusterIP string) {
	t.Helper()
	_, err := p.Cluster.CoreV1().Services(p.Namespace).Create(context.TODO(), &ac.Service{
		ObjectMeta: am.ObjectMeta{Name: name, Namespace: p.Namespace},
		Spec:       ac.ServiceSpec{ClusterIP: clusterIP},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func createHairpinWebhook(p *k8s.Provider, name, url string) error {
	_, err := p.SystemResourceCreate("webhook", structs.ResourceCreateOptions{
		Name:       options.String(name),
		Parameters: map[string]string{"Url": url},
	})
	return err
}

func TestWebhookCreateRouterHairpin(t *testing.T) {
	defer k8s.UseDefaultWebhookSSRFValidatorForTest()()
	defer k8s.SetWebhookLookupIPForTest(func(host string) ([]net.IP, error) {
		answers := map[string][]string{
			"console.example.com":       {hairpinRouterIP},
			"mixed-private.example.com": {hairpinRouterIP, "10.0.0.5"},
			"mixed-public.example.com":  {hairpinRouterIP, "203.0.113.5"},
			"private.example.com":       {"10.0.0.5"},
			"web.example.com":           {"172.20.10.10"},
		}
		a, ok := answers[host]
		if !ok {
			return nil, errors.New("no such host")
		}
		ips := make([]net.IP, 0, len(a))
		for _, s := range a {
			ips = append(ips, net.ParseIP(s))
		}
		return ips, nil
	})()

	testProvider(t, func(p *k8s.Provider) {
		err := createHairpinWebhook(p, "no-router", "https://console.example.com/racks/r1/events")
		require.ErrorContains(t, err, "non-routable")

		createHairpinService(t, p, "router", hairpinRouterIP)
		createHairpinService(t, p, "web", "172.20.10.10")

		require.NoError(t, createHairpinWebhook(p, "hairpin", "https://console.example.com/racks/r1/events"))

		refused := []struct{ name, url string }{
			{"http", "http://console.example.com/racks/r1/events"},
			{"mixed-private", "https://mixed-private.example.com/x"},
			{"mixed-public", "https://mixed-public.example.com/x"},
			{"private", "https://private.example.com/x"},
			{"literal", "https://" + hairpinRouterIP + "/x"},
			{"other-service", "https://web.example.com/x"},
		}
		for _, c := range refused {
			t.Run(c.name, func(t *testing.T) {
				require.ErrorContains(t, createHairpinWebhook(p, c.name, c.url), "non-routable")
			})
		}

		p.ProxyProtocol = true
		err = createHairpinWebhook(p, "proxy-protocol", "https://console.example.com/racks/r1/events")
		p.ProxyProtocol = false
		require.ErrorContains(t, err, "non-routable")
	})
}

func TestWebhookDispatchRouterHairpin(t *testing.T) {
	defer k8s.UseDefaultWebhookSSRFValidatorForTest()()

	var answer atomic.Value
	answer.Store(hairpinRouterIP)
	defer k8s.SetWebhookLookupIPForTest(func(host string) ([]net.IP, error) {
		if host != "console.example.com" {
			return nil, errors.New("no such host")
		}
		s, _ := answer.Load().(string)
		return []net.IP{net.ParseIP(s)}, nil
	})()

	hits := make(chan string, 4)
	defer k8s.SetDispatchWebhookSignedFnForTest(func(url string, _ []byte, _ [][]byte, _ time.Duration) error {
		hits <- url
		return nil
	})()

	testProvider(t, func(p *k8s.Provider) {
		createHairpinService(t, p, "router", hairpinRouterIP)

		u := "https://console.example.com/racks/r1/events"
		require.NoError(t, createHairpinWebhook(p, "console", u))

		send := func() {
			require.NoError(t, p.EventSend("app:create", structs.EventSendOptions{Data: map[string]string{"app": "demo"}}))
		}
		refused := func() {
			t.Helper()
			read := captureStdout(t)
			send()
			drainPendingDispatches()
			assert.Contains(t, read(), "at=ssrf_blocked url_host=console.example.com")
			assert.Len(t, hits, 0)
		}

		send()
		select {
		case got := <-hits:
			assert.Equal(t, u, got)
		case <-time.After(2 * time.Second):
			t.Fatal("hairpinned webhook was not dispatched")
		}

		answer.Store("10.0.0.5")
		refused()

		k8s.ResetWebhookSSRFLoggedForTest()
		answer.Store(hairpinRouterIP)
		svc, err := p.Cluster.CoreV1().Services(p.Namespace).Get(context.TODO(), "router", am.GetOptions{})
		require.NoError(t, err)
		svc.Spec.ClusterIP = "172.20.1.1"
		_, err = p.Cluster.CoreV1().Services(p.Namespace).Update(context.TODO(), svc, am.UpdateOptions{})
		require.NoError(t, err)
		refused()
	})
}
