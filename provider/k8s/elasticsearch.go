package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/convox/convox/pkg/elastic"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	elasticsearchClaimPrefix = "elasticsearch-data-elasticsearch-"
	elasticsearchLease       = "elasticsearch-maintenance"
)

var elasticsearchMinDisk = resource.MustParse("5Gi")

type volumeExpander struct {
	p       *Provider
	size    resource.Quantity
	logged  map[string]bool
	patched map[string]time.Time
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

			if x != nil {
				x.expand(tctx, time.Now())
			}

			cancel()

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

	return &volumeExpander{p: p, size: size, logged: map[string]bool{}, patched: map[string]time.Time{}}
}

func (x *volumeExpander) expand(ctx context.Context, now time.Time) {
	pvcs, err := x.p.Cluster.CoreV1().PersistentVolumeClaims(x.p.Namespace).List(ctx, am.ListOptions{})
	if err != nil {
		fmt.Printf("ns=elastic at=expand error=%q\n", err)
		return
	}

	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]

		if !strings.HasPrefix(pvc.Name, elasticsearchClaimPrefix) {
			continue
		}

		requested := pvc.Spec.Resources.Requests[ac.ResourceStorage]

		if requested.Cmp(x.size) >= 0 {
			capacity := pvc.Status.Capacity[ac.ResourceStorage]
			key := "lag/" + pvc.Name + "/" + requested.String()

			if at, ok := x.patched[pvc.Name]; ok && capacity.Cmp(requested) < 0 && now.Sub(at) >= 10*time.Minute && !x.logged[key] {
				fmt.Printf("ns=elastic at=expand claim=%s requested=%s capacity=%s warning=%q\n", pvc.Name, requested.String(), capacity.String(), "volume has not grown yet")
				x.logged[key] = true
			}

			continue
		}

		patch := fmt.Sprintf(`{"spec":{"resources":{"requests":{"storage":%q}}}}`, x.size.String())

		if _, err := x.p.Cluster.CoreV1().PersistentVolumeClaims(x.p.Namespace).Patch(ctx, pvc.Name, types.MergePatchType, []byte(patch), am.PatchOptions{}); err != nil {
			key := "patch/" + pvc.Name + "/" + x.size.String()

			if !x.logged[key] {
				fmt.Printf("ns=elastic at=expand claim=%s error=%q\n", pvc.Name, err)
				x.logged[key] = true
			}

			continue
		}

		x.patched[pvc.Name] = now
		fmt.Printf("ns=elastic at=expand claim=%s from=%s to=%s\n", pvc.Name, requested.String(), x.size.String())
	}
}
