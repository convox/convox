package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/elastic"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	elasticsearchClaimPrefix = "elasticsearch-data-elasticsearch-"
	elasticsearchLease       = "elasticsearch-maintenance"
	elasticsearchResizeKey   = "convox.com/elasticsearch-resize"
	elasticsearchStatefulSet = "elasticsearch"
	resizeLag                = 10 * time.Minute
	resizeTimeout            = 30 * time.Minute
)

var elasticsearchMinDisk = resource.MustParse("5Gi")

type volumeExpander struct {
	p       *Provider
	size    resource.Quantity
	logged  map[string]bool
	lagging map[string]time.Time
}

type resizeRecord struct {
	Size       string    `json:"size"`
	Replicas   int32     `json:"replicas"`
	Generation int64     `json:"generation"`
	Stopped    time.Time `json:"stopped"`
}

func (p *Provider) RunElasticsearchMaintenance(ec *elastic.Client) {
	x := p.volumeExpander(os.Getenv("ELASTICSEARCH_DISK"))

	elect := func(onStarted func(context.Context), onStopped func()) error {
		return RunUsingLeaderElection(context.Background(), p.Namespace, elasticsearchLease, p.Cluster, onStarted, onStopped)
	}

	go campaign(elect, func(ctx context.Context) {
		r := ec.Retainer(p.Name)

		for {
			tctx, cancel := context.WithTimeout(ctx, 30*time.Second)

			if err := r.Tick(tctx, time.Now().UTC()); err != nil {
				fmt.Printf("ns=elastic at=retain error=%q\n", err)
			}

			cancel()

			if x != nil {
				xctx, xcancel := context.WithTimeout(ctx, 30*time.Second)
				x.expand(xctx, time.Now())
				xcancel()
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	})
}

// campaign runs work for each term of leadership, and only starts campaigning again
// once the previous term's work has returned.
func campaign(elect func(onStarted func(context.Context), onStopped func()) error, work func(context.Context)) {
	for {
		done := make(chan struct{})
		stopped := make(chan struct{})

		err := elect(func(ctx context.Context) {
			defer close(done)
			work(ctx)
		}, func() {
			close(stopped)
		})
		if err != nil {
			fmt.Printf("ns=elastic at=maintenance error=%q\n", err)
			return
		}

		<-stopped
		<-done

		fmt.Println("ns=elastic at=lost_leadership")
	}
}

func (p *Provider) volumeExpander(disk string) *volumeExpander {
	if disk == "" {
		return nil
	}

	size, err := resource.ParseQuantity(disk)
	if err != nil || size.Cmp(elasticsearchMinDisk) < 0 {
		fmt.Printf("ns=elastic at=expand error=%q\n", fmt.Sprintf("ignoring elasticsearch_disk %q: must be at least 5Gi", disk))
		return nil
	}

	return &volumeExpander{p: p, size: size, logged: map[string]bool{}, lagging: map[string]time.Time{}}
}

func (x *volumeExpander) expand(ctx context.Context, now time.Time) {
	pvcs, err := x.p.Cluster.CoreV1().PersistentVolumeClaims(x.p.Namespace).List(ctx, am.ListOptions{})
	if err != nil {
		fmt.Printf("ns=elastic at=expand error=%q\n", err)
		return
	}

	sts, err := x.p.Cluster.AppsV1().StatefulSets(x.p.Namespace).Get(ctx, elasticsearchStatefulSet, am.GetOptions{})
	if err != nil {
		x.logOnce("statefulset", fmt.Sprintf("ns=elastic at=expand error=%q", err))
		sts = nil
	}

	rec := resizeRecordOf(pvcs.Items)

	if sts != nil && rec.Replicas > 0 {
		if replicasOf(sts) == 0 {
			x.resume(ctx, now, sts, pvcs.Items, rec)
			return
		}

		tried := &resizeRecord{Size: rec.Size}

		if sts.Generation == rec.Generation {
			tried = nil
			rec = resizeRecord{}
		}

		if err := x.setRecord(ctx, tried); err != nil {
			fmt.Printf("ns=elastic at=expand error=%q\n", err)
		}
	}

	lagging := false

	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]

		if !strings.HasPrefix(pvc.Name, elasticsearchClaimPrefix) {
			continue
		}

		requested := pvc.Spec.Resources.Requests[ac.ResourceStorage]

		if requested.Cmp(x.size) < 0 {
			x.raise(ctx, now, pvc, requested)
			continue
		}

		capacity, ok := pvc.Status.Capacity[ac.ResourceStorage]

		if !ok || !counted(pvc, replicasOf(sts)) || capacity.Cmp(requested) >= 0 {
			delete(x.lagging, pvc.Name)
			continue
		}

		since, ok := x.lagging[pvc.Name]
		if !ok {
			x.lagging[pvc.Name] = now
			continue
		}

		if now.Sub(since) < resizeLag {
			continue
		}

		x.logOnce("lag/"+pvc.Name+"/"+requested.String(), fmt.Sprintf("ns=elastic at=expand claim=%s requested=%s capacity=%s warning=%q", pvc.Name, requested.String(), capacity.String(), "volume has not grown yet"))
		lagging = true
	}

	if lagging && rec.Size != x.size.String() {
		x.stop(ctx, now, sts)
	}
}

func (x *volumeExpander) raise(ctx context.Context, now time.Time, pvc *ac.PersistentVolumeClaim, requested resource.Quantity) {
	patch := fmt.Sprintf(`{"spec":{"resources":{"requests":{"storage":%q}}}}`, x.size.String())

	if _, err := x.p.Cluster.CoreV1().PersistentVolumeClaims(x.p.Namespace).Patch(ctx, pvc.Name, types.MergePatchType, []byte(patch), am.PatchOptions{}); err != nil {
		x.logOnce("patch/"+pvc.Name+"/"+x.size.String(), fmt.Sprintf("ns=elastic at=expand claim=%s error=%q", pvc.Name, err))
		return
	}

	x.lagging[pvc.Name] = now
	fmt.Printf("ns=elastic at=expand claim=%s from=%s to=%s\n", pvc.Name, requested.String(), x.size.String())
}

func (x *volumeExpander) stop(ctx context.Context, now time.Time, sts *appsv1.StatefulSet) {
	replicas := replicasOf(sts)

	if err := x.setRecord(ctx, &resizeRecord{Size: x.size.String(), Replicas: replicas, Generation: sts.Generation, Stopped: now.UTC()}); err != nil {
		x.logOnce("stop/"+x.size.String(), fmt.Sprintf("ns=elastic at=expand error=%q", err))
		return
	}

	if err := x.scale(ctx, 0); err != nil {
		x.logOnce("stop/"+x.size.String(), fmt.Sprintf("ns=elastic at=expand error=%q", err))
		return
	}

	fmt.Printf("ns=elastic at=expand stopping=elasticsearch replicas=%d\n", replicas)
}

func (x *volumeExpander) resume(ctx context.Context, now time.Time, sts *appsv1.StatefulSet, pvcs []ac.PersistentVolumeClaim, rec resizeRecord) {
	grown := sts.Status.Replicas == 0

	for i := range pvcs {
		pvc := &pvcs[i]

		if strings.HasPrefix(pvc.Name, elasticsearchClaimPrefix) && counted(pvc, rec.Replicas) && !resized(pvc) {
			grown = false
		}
	}

	if !grown && now.Sub(rec.Stopped) < resizeTimeout {
		return
	}

	if err := x.scale(ctx, rec.Replicas); err != nil {
		fmt.Printf("ns=elastic at=expand error=%q\n", err)
		return
	}

	if err := x.setRecord(ctx, &resizeRecord{Size: rec.Size}); err != nil {
		fmt.Printf("ns=elastic at=expand error=%q\n", err)
	}

	if grown {
		fmt.Printf("ns=elastic at=expand starting=elasticsearch replicas=%d\n", rec.Replicas)
	} else {
		fmt.Printf("ns=elastic at=expand starting=elasticsearch replicas=%d warning=%q\n", rec.Replicas, "volumes did not grow while detached")
	}
}

func (x *volumeExpander) scale(ctx context.Context, replicas int32) error {
	patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)

	_, err := x.p.Cluster.AppsV1().StatefulSets(x.p.Namespace).Patch(ctx, elasticsearchStatefulSet, types.MergePatchType, []byte(patch), am.PatchOptions{})

	return err
}

func (x *volumeExpander) setRecord(ctx context.Context, rec *resizeRecord) error {
	value := []byte("null")

	if rec != nil {
		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}

		if value, err = json.Marshal(string(data)); err != nil {
			return err
		}
	}

	patch := fmt.Sprintf(`{"metadata":{"annotations":{%q:%s}}}`, elasticsearchResizeKey, value)

	_, err := x.p.Cluster.CoreV1().PersistentVolumeClaims(x.p.Namespace).Patch(ctx, elasticsearchClaimPrefix+"0", types.MergePatchType, []byte(patch), am.PatchOptions{})

	return err
}

func (x *volumeExpander) logOnce(key, line string) {
	if !x.logged[key] {
		fmt.Println(line)
		x.logged[key] = true
	}
}

func resizeRecordOf(pvcs []ac.PersistentVolumeClaim) resizeRecord {
	var rec resizeRecord

	for i := range pvcs {
		if pvcs[i].Name == elasticsearchClaimPrefix+"0" {
			if err := json.Unmarshal([]byte(pvcs[i].Annotations[elasticsearchResizeKey]), &rec); err != nil {
				return resizeRecord{}
			}
		}
	}

	return rec
}

func replicasOf(sts *appsv1.StatefulSet) int32 {
	if sts == nil || sts.Spec.Replicas == nil {
		return 0
	}

	return *sts.Spec.Replicas
}

func counted(pvc *ac.PersistentVolumeClaim, replicas int32) bool {
	n, err := strconv.Atoi(strings.TrimPrefix(pvc.Name, elasticsearchClaimPrefix))

	return err == nil && pvc.Status.Phase == ac.ClaimBound && n < int(replicas)
}

func resized(pvc *ac.PersistentVolumeClaim) bool {
	for _, c := range pvc.Status.Conditions {
		if c.Type == ac.PersistentVolumeClaimFileSystemResizePending && c.Status == ac.ConditionTrue {
			return true
		}
	}

	capacity := pvc.Status.Capacity[ac.ResourceStorage]
	requested := pvc.Spec.Resources.Requests[ac.ResourceStorage]

	return capacity.Cmp(requested) >= 0
}
