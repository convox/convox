package k8s

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
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

func boundClaim(name, requested, capacity string, conditions ...ac.PersistentVolumeClaimConditionType) *ac.PersistentVolumeClaim {
	c := claim(name, requested)
	c.Status.Phase = ac.ClaimBound
	c.Status.Capacity = ac.ResourceList{ac.ResourceStorage: resource.MustParse(capacity)}

	for _, t := range conditions {
		c.Status.Conditions = append(c.Status.Conditions, ac.PersistentVolumeClaimCondition{Type: t, Status: ac.ConditionTrue})
	}

	return c
}

func withRecord(c *ac.PersistentVolumeClaim, rec resizeRecord) *ac.PersistentVolumeClaim {
	data, err := json.Marshal(rec)
	if err != nil {
		panic(err)
	}

	c.Annotations = map[string]string{elasticsearchResizeKey: string(data)}

	return c
}

func elasticsearchSet(spec, status int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: am.ObjectMeta{Name: "elasticsearch", Namespace: "rack-system", Generation: 1},
		Spec:       appsv1.StatefulSetSpec{Replicas: &spec},
		Status:     appsv1.StatefulSetStatus{Replicas: status},
	}
}

func replicas(t *testing.T, p *Provider) int32 {
	t.Helper()

	s, err := p.Cluster.AppsV1().StatefulSets("rack-system").Get(context.TODO(), "elasticsearch", am.GetOptions{})
	require.NoError(t, err)

	return *s.Spec.Replicas
}

func storedRecord(t *testing.T, p *Provider) resizeRecord {
	t.Helper()

	pvc, err := p.Cluster.CoreV1().PersistentVolumeClaims("rack-system").Get(context.TODO(), "elasticsearch-data-elasticsearch-0", am.GetOptions{})
	require.NoError(t, err)

	var rec resizeRecord

	if v, ok := pvc.Annotations[elasticsearchResizeKey]; ok {
		require.NoError(t, json.Unmarshal([]byte(v), &rec))
	}

	return rec
}

func TestVolumeExpanderStopsWhenClaimsDoNotGrow(t *testing.T) {
	cs := fake.NewSimpleClientset(
		boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi"),
		boundClaim("elasticsearch-data-elasticsearch-1", "10Gi", "5Gi"),
		elasticsearchSet(2, 2),
	)

	p := &Provider{Namespace: "rack-system", Cluster: cs}

	x := p.volumeExpander("10Gi")
	t0 := time.Now()

	out := captureStdout(t, func() {
		x.expand(context.TODO(), t0)
		x.expand(context.TODO(), t0.Add(9*time.Minute))
	})

	require.Empty(t, out)
	require.Equal(t, int32(2), replicas(t, p))

	out = captureStdout(t, func() {
		x.expand(context.TODO(), t0.Add(10*time.Minute))
		x.expand(context.TODO(), t0.Add(11*time.Minute))
	})

	require.Equal(t, 2, strings.Count(out, `warning="volume has not grown yet"`))
	require.Equal(t, 1, strings.Count(out, "at=expand stopping=elasticsearch replicas=2\n"))
	require.Equal(t, int32(0), replicas(t, p))

	rec := storedRecord(t, p)
	require.Equal(t, "10Gi", rec.Size)
	require.Equal(t, int32(2), rec.Replicas)
	require.Equal(t, int64(1), rec.Generation)
	require.True(t, rec.Stopped.Equal(t0.Add(10*time.Minute)))

	var order []string

	for _, a := range cs.Actions() {
		if a.GetVerb() == "patch" {
			order = append(order, a.GetResource().Resource)
		}
	}

	require.Equal(t, []string{"persistentvolumeclaims", "statefulsets"}, order)
}

func TestVolumeExpanderStopsForClaimsAboveTarget(t *testing.T) {
	p := &Provider{Namespace: "rack-system", Cluster: fake.NewSimpleClientset(
		boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi"),
		boundClaim("elasticsearch-data-elasticsearch-1", "10Gi", "5Gi"),
		elasticsearchSet(2, 2),
	)}

	x := p.volumeExpander("8Gi")
	t0 := time.Now()

	x.expand(context.TODO(), t0)
	x.expand(context.TODO(), t0.Add(10*time.Minute))

	require.Equal(t, "10Gi", requested(t, p, "elasticsearch-data-elasticsearch-0"))
	require.Equal(t, int32(0), replicas(t, p))
	require.Equal(t, "8Gi", storedRecord(t, p).Size)
}

func TestVolumeExpanderIgnoresUncountedClaims(t *testing.T) {
	pending := claim("elasticsearch-data-elasticsearch-1", "10Gi")
	pending.Status = ac.PersistentVolumeClaimStatus{Phase: ac.ClaimPending}

	p := &Provider{Namespace: "rack-system", Cluster: fake.NewSimpleClientset(
		boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "10Gi"),
		pending,
		boundClaim("elasticsearch-data-elasticsearch-2", "10Gi", "5Gi", ac.PersistentVolumeClaimFileSystemResizePending),
		elasticsearchSet(2, 2),
	)}

	x := p.volumeExpander("10Gi")
	t0 := time.Now()

	out := captureStdout(t, func() {
		x.expand(context.TODO(), t0)
		x.expand(context.TODO(), t0.Add(11*time.Minute))
	})

	require.Empty(t, out)
	require.Equal(t, int32(2), replicas(t, p))
	require.Equal(t, resizeRecord{}, storedRecord(t, p))
}

func TestVolumeExpanderLeavesStoppedSetAlone(t *testing.T) {
	p := &Provider{Namespace: "rack-system", Cluster: fake.NewSimpleClientset(
		boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi"),
		elasticsearchSet(0, 0),
	)}

	x := p.volumeExpander("10Gi")
	t0 := time.Now()

	x.expand(context.TODO(), t0)
	x.expand(context.TODO(), t0.Add(11*time.Minute))

	require.Equal(t, int32(0), replicas(t, p))
	require.Equal(t, resizeRecord{}, storedRecord(t, p))
}

func TestVolumeExpanderStopThatNeverLanded(t *testing.T) {
	cs := fake.NewSimpleClientset(
		boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi"),
		elasticsearchSet(1, 1),
	)
	cs.PrependReactor("patch", "statefulsets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("conflict")
	})

	p := &Provider{Namespace: "rack-system", Cluster: cs}
	x := p.volumeExpander("10Gi")
	t0 := time.Now()

	out := captureStdout(t, func() {
		x.expand(context.TODO(), t0)
		x.expand(context.TODO(), t0.Add(10*time.Minute))
		x.expand(context.TODO(), t0.Add(11*time.Minute))
	})

	require.Equal(t, 1, strings.Count(out, `at=expand error="conflict"`))
	require.NotContains(t, out, "stopping=")
	require.Equal(t, int32(1), replicas(t, p))
	require.Equal(t, int32(1), storedRecord(t, p).Replicas)
}

func TestVolumeExpanderStopThatLandedWithError(t *testing.T) {
	cs := fake.NewSimpleClientset(
		boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi"),
		elasticsearchSet(1, 0),
	)

	failed := false

	cs.PrependReactor("patch", "statefulsets", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if failed {
			return false, nil, nil
		}

		failed = true

		if _, _, err := k8stesting.ObjectReaction(cs.Tracker())(a); err != nil {
			return true, nil, err
		}

		return true, nil, errors.New("connection lost")
	})

	p := &Provider{Namespace: "rack-system", Cluster: cs}
	x := p.volumeExpander("10Gi")
	t0 := time.Now()

	x.expand(context.TODO(), t0)
	x.expand(context.TODO(), t0.Add(10*time.Minute))

	require.Equal(t, int32(0), replicas(t, p))
	require.Equal(t, int32(1), storedRecord(t, p).Replicas)

	x.expand(context.TODO(), t0.Add(40*time.Minute))

	require.Equal(t, int32(1), replicas(t, p))
	require.Equal(t, resizeRecord{Size: "10Gi"}, storedRecord(t, p))
}

func TestVolumeExpanderResumesWhenGrown(t *testing.T) {
	t0 := time.Now()

	p := &Provider{Namespace: "rack-system", Cluster: fake.NewSimpleClientset(
		withRecord(boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi", ac.PersistentVolumeClaimFileSystemResizePending), resizeRecord{Size: "10Gi", Replicas: 2, Stopped: t0}),
		boundClaim("elasticsearch-data-elasticsearch-1", "10Gi", "5Gi", ac.PersistentVolumeClaimFileSystemResizePending),
		elasticsearchSet(0, 0),
	)}

	x := p.volumeExpander("10Gi")

	out := captureStdout(t, func() {
		x.expand(context.TODO(), t0.Add(5*time.Minute))
	})

	require.Equal(t, "ns=elastic at=expand starting=elasticsearch replicas=2\n", out)
	require.Equal(t, int32(2), replicas(t, p))
	require.Equal(t, resizeRecord{Size: "10Gi"}, storedRecord(t, p))

	out = captureStdout(t, func() {
		x.expand(context.TODO(), t0.Add(6*time.Minute))
		x.expand(context.TODO(), t0.Add(20*time.Minute))
	})

	require.Equal(t, 2, strings.Count(out, `warning="volume has not grown yet"`))
	require.NotContains(t, out, "stopping=")
	require.Equal(t, int32(2), replicas(t, p))
}

func TestVolumeExpanderWaitsForPodsToStop(t *testing.T) {
	t0 := time.Now()

	p := &Provider{Namespace: "rack-system", Cluster: fake.NewSimpleClientset(
		withRecord(boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi", ac.PersistentVolumeClaimFileSystemResizePending), resizeRecord{Size: "10Gi", Replicas: 2, Stopped: t0}),
		boundClaim("elasticsearch-data-elasticsearch-1", "10Gi", "5Gi", ac.PersistentVolumeClaimFileSystemResizePending),
		elasticsearchSet(0, 1),
	)}

	p.volumeExpander("10Gi").expand(context.TODO(), t0.Add(5*time.Minute))

	require.Equal(t, int32(0), replicas(t, p))
	require.Equal(t, int32(2), storedRecord(t, p).Replicas)
}

func TestVolumeExpanderResumesPastPendingClaim(t *testing.T) {
	t0 := time.Now()

	pending := claim("elasticsearch-data-elasticsearch-1", "5Gi")
	pending.Status = ac.PersistentVolumeClaimStatus{Phase: ac.ClaimPending}

	p := &Provider{Namespace: "rack-system", Cluster: fake.NewSimpleClientset(
		withRecord(boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi", ac.PersistentVolumeClaimFileSystemResizePending), resizeRecord{Size: "10Gi", Replicas: 2, Stopped: t0}),
		pending,
		elasticsearchSet(0, 0),
	)}

	p.volumeExpander("10Gi").expand(context.TODO(), t0.Add(5*time.Minute))

	require.Equal(t, int32(2), replicas(t, p))
}

func TestVolumeExpanderResumesAfterTimeout(t *testing.T) {
	t0 := time.Now()

	p := &Provider{Namespace: "rack-system", Cluster: fake.NewSimpleClientset(
		withRecord(boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi"), resizeRecord{Size: "10Gi", Replicas: 2, Stopped: t0}),
		boundClaim("elasticsearch-data-elasticsearch-1", "10Gi", "5Gi"),
		elasticsearchSet(0, 0),
	)}

	x := p.volumeExpander("10Gi")

	out := captureStdout(t, func() {
		x.expand(context.TODO(), t0.Add(29*time.Minute))
	})

	require.Empty(t, out)
	require.Equal(t, int32(0), replicas(t, p))

	out = captureStdout(t, func() {
		x.expand(context.TODO(), t0.Add(30*time.Minute))
	})

	require.Equal(t, "ns=elastic at=expand starting=elasticsearch replicas=2 warning=\"volumes did not grow while detached\"\n", out)
	require.Equal(t, int32(2), replicas(t, p))
	require.Equal(t, resizeRecord{Size: "10Gi"}, storedRecord(t, p))
}

func TestVolumeExpanderRecordAfterRestoredElsewhere(t *testing.T) {
	t0 := time.Now()

	restored := elasticsearchSet(2, 2)
	restored.Generation = 3

	cs := fake.NewSimpleClientset(
		withRecord(boundClaim("elasticsearch-data-elasticsearch-0", "10Gi", "5Gi"), resizeRecord{Size: "10Gi", Replicas: 2, Generation: 1, Stopped: t0}),
		restored,
	)

	p := &Provider{Namespace: "rack-system", Cluster: cs}
	x := p.volumeExpander("10Gi")

	x.expand(context.TODO(), t0.Add(time.Minute))
	x.expand(context.TODO(), t0.Add(12*time.Minute))

	require.Equal(t, resizeRecord{Size: "10Gi"}, storedRecord(t, p))
	require.Equal(t, int32(2), replicas(t, p))

	for _, a := range cs.Actions() {
		require.False(t, a.GetVerb() == "patch" && a.GetResource().Resource == "statefulsets")
	}
}
