package k8s

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func claim(name, size string) *ac.PersistentVolumeClaim {
	return &ac.PersistentVolumeClaim{
		ObjectMeta: am.ObjectMeta{Name: name, Namespace: "rack-system"},
		Spec: ac.PersistentVolumeClaimSpec{
			Resources: ac.VolumeResourceRequirements{Requests: ac.ResourceList{ac.ResourceStorage: resource.MustParse(size)}},
		},
		Status: ac.PersistentVolumeClaimStatus{Capacity: ac.ResourceList{ac.ResourceStorage: resource.MustParse(size)}},
	}
}

func requested(t *testing.T, p *Provider, name string) string {
	t.Helper()

	pvc, err := p.Cluster.CoreV1().PersistentVolumeClaims("rack-system").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)

	q := pvc.Spec.Resources.Requests[ac.ResourceStorage]

	return q.String()
}

func TestVolumeExpanderValue(t *testing.T) {
	p := &Provider{Namespace: "rack-system"}

	for _, v := range []string{"", "4Gi", "5G", "abc"} {
		require.Nil(t, p.volumeExpander(v), v)
	}

	require.NotNil(t, p.volumeExpander("5Gi"))
	require.NotNil(t, p.volumeExpander("120Gi"))
}

func TestVolumeExpanderExpandsSmallerClaims(t *testing.T) {
	p := &Provider{Namespace: "rack-system", Cluster: fake.NewSimpleClientset(
		claim("elasticsearch-data-elasticsearch-0", "5Gi"),
		claim("elasticsearch-data-elasticsearch-1", "20Gi"),
		claim("registry", "1Gi"),
	)}

	p.volumeExpander("10Gi").expand(context.TODO(), time.Now())

	require.Equal(t, "10Gi", requested(t, p, "elasticsearch-data-elasticsearch-0"))
	require.Equal(t, "20Gi", requested(t, p, "elasticsearch-data-elasticsearch-1"))
	require.Equal(t, "1Gi", requested(t, p, "registry"))
}

func TestVolumeExpanderRefusedPatch(t *testing.T) {
	cs := fake.NewSimpleClientset(claim("elasticsearch-data-elasticsearch-0", "5Gi"))
	cs.PrependReactor("patch", "persistentvolumeclaims", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("only dynamically provisioned pvc can be resized")
	})

	p := &Provider{Namespace: "rack-system", Cluster: cs}
	x := p.volumeExpander("10Gi")

	out := captureStdout(t, func() {
		x.expand(context.TODO(), time.Now())
		x.expand(context.TODO(), time.Now())
	})

	require.Equal(t, 1, strings.Count(out, "at=expand claim=elasticsearch-data-elasticsearch-0 error="))
	require.Equal(t, "5Gi", requested(t, p, "elasticsearch-data-elasticsearch-0"))
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	stdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = stdout }()

	fn()

	require.NoError(t, w.Close())

	data, err := io.ReadAll(r)
	require.NoError(t, err)

	return string(data)
}

func TestCampaignRestartsWithoutOverlap(t *testing.T) {
	var active, terms int32

	elections := 0

	elect := func(onStarted func(context.Context), onStopped func()) error {
		elections++
		if elections > 3 {
			return errors.New("stop")
		}

		ctx, cancel := context.WithCancel(context.Background())

		go onStarted(ctx)

		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
			onStopped()
		}()

		return nil
	}

	campaign(elect, func(ctx context.Context) {
		require.Equal(t, int32(1), atomic.AddInt32(&active, 1))
		atomic.AddInt32(&terms, 1)
		<-ctx.Done()
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&active, -1)
	})

	require.Equal(t, int32(3), atomic.LoadInt32(&terms))
	require.Equal(t, int32(0), atomic.LoadInt32(&active))
}
