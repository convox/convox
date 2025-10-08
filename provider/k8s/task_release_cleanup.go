package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	convoxv1 "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cv "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned"
	"github.com/convox/logger"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const cleanupAnnotationKey = "convox.io/last-release-build-cleanup"

type providerForReleaseCleaner interface {
	AppList() (structs.Apps, error)
	AppNamespace(appName string) string
}

type releaseCleaner struct {
	provider                        providerForReleaseCleaner
	engine                          Engine
	convox                          cv.Interface
	logger                          *logger.Logger
	cluster                         kubernetes.Interface
	systemNamespace                 string
	ctx                             context.Context
	releasesToRetainAfterActive     int
	releaseBuildCleanupIntervalHour int
}

func (a *releaseCleaner) Run() error {
	a.logger.Logf("starting release build cleanup task...")
	if a.releaseBuildCleanupIntervalHour <= 0 {
		a.releaseBuildCleanupIntervalHour = 24
	}
	if err := a.waitUntilScheduledForCleanup(); err != nil {
		return err
	}
	return a.cleanupReleasesAndBuilds()
}

func (a *releaseCleaner) waitUntilScheduledForCleanup() error {
	sysNs, err := a.cluster.CoreV1().Namespaces().Get(a.ctx, a.systemNamespace, v1.GetOptions{})
	if err != nil {
		a.logger.Errorf("failed to get system namespace: %s", err)
		return err
	}

	if sysNs.Annotations[cleanupAnnotationKey] != "" {
		t, err := time.Parse(time.RFC3339, sysNs.Annotations[cleanupAnnotationKey])
		if err == nil && t.Add(time.Duration(a.releaseBuildCleanupIntervalHour)*time.Hour).After(time.Now().UTC()) {
			a.logger.Logf("release build cleanup already run in last %d hours, skipping", a.releaseBuildCleanupIntervalHour)
			time.Sleep(t.Add(time.Duration(a.releaseBuildCleanupIntervalHour)*time.Hour).Sub(time.Now().UTC()) + (10 * time.Second))
		}
	}
	return nil
}

func (a *releaseCleaner) cleanupReleasesAndBuilds() error {
	a.logger.Logf("running release build cleanup...")

	appList, err := a.provider.AppList()
	if err != nil {
		a.logger.Errorf("failed to list apps: %s", err)
		time.Sleep(3 * time.Second)
		return err
	}

	for _, app := range appList {
		if app.Release == "" {
			continue
		}

		if err := a.appReleaseAndBuildCleanup(&app); err != nil {
			a.logger.Errorf("failed to cleanup release builds for app '%s': %s", app.Name, err)
		}
		time.Sleep(3 * time.Second)
	}

	patchBytes, err := patchBytes(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				cleanupAnnotationKey: time.Now().UTC().Format(time.RFC3339),
			},
		},
	})
	if err != nil {
		a.logger.Errorf("failed to create patch bytes err=%q\n", err)
		return err
	}
	_, err = a.cluster.CoreV1().Namespaces().Patch(a.ctx, a.systemNamespace, types.MergePatchType, patchBytes, am.PatchOptions{})
	if err != nil {
		a.logger.Errorf("failed to patch namespace '%s' err=%q\n", a.systemNamespace, err)
	}
	return err
}

func (a *releaseCleaner) toTime(t string) time.Time {
	parsedTime, _ := time.Parse(common.SortableTime, t)
	return parsedTime
}

func (a *releaseCleaner) appReleaseAndBuildCleanup(app *structs.App) error {
	listOpts := am.ListOptions{}
	bs := []convoxv1.Build{}

	appNamespace := ""
	if app.Tags != nil && app.Tags["namespace"] != "" {
		appNamespace = app.Tags["namespace"]
	} else {
		appNamespace = a.provider.AppNamespace(app.Name)
	}

	for {
		bList, err := a.convox.ConvoxV1().Builds(appNamespace).List(listOpts)
		if err != nil {
			return err
		}

		bs = append(bs, bList.Items...)

		if bList.GetContinue() == "" {
			break
		}
		listOpts.Continue = bList.GetContinue()
	}

	listOpts = am.ListOptions{}
	rs := []convoxv1.Release{}

	for {
		rList, err := a.convox.ConvoxV1().Releases(appNamespace).List(listOpts)
		if err != nil {
			return err
		}

		rs = append(rs, rList.Items...)

		if rList.GetContinue() == "" {
			break
		}
		listOpts.Continue = rList.GetContinue()
	}

	sort.Slice(rs, func(i, j int) bool {
		// sort by creation time, so that the newest release comes first
		return a.toTime(rs[i].Spec.Created).After(a.toTime(rs[j].Spec.Created))
	})

	foundActiveIndex := -1
	for i, r := range rs {
		if r.Name == strings.ToLower(app.Release) {
			foundActiveIndex = i
			break
		}
	}

	if foundActiveIndex == -1 {
		a.logger.Logf("active release '%s' not found for app '%s', skipping build cleanup", app.Release, app.Name)
		return nil
	}

	buildToKeep := map[string]struct{}{}
	for i := 0; i < (foundActiveIndex+a.releasesToRetainAfterActive+1) && i < len(rs); i++ {
		if rs[i].Spec.Build != "" {
			buildToKeep[strings.ToLower(rs[i].Spec.Build)] = struct{}{}
		}
	}

	for i := foundActiveIndex + a.releasesToRetainAfterActive + 1; i < len(rs); i++ {
		a.convox.ConvoxV1().Releases(appNamespace).Delete(rs[i].Name, &am.DeleteOptions{})
		time.Sleep(50 * time.Millisecond) // to avoid rate limit
	}

	oldReleaseTime := a.toTime(rs[min(foundActiveIndex+a.releasesToRetainAfterActive, len(rs)-1)].Spec.Created)
	oldestReleaseTimeToKeep := oldReleaseTime.Add(-1 * time.Minute)
	buildToDelete := []string{}
	buildTagsToDelete := []string{}
	for _, b := range bs {
		// also ensure we don't delete builds that are newer than the oldest release we are keeping
		if _, ok := buildToKeep[b.Name]; !ok && a.toTime(b.Spec.Started).Before(oldestReleaseTimeToKeep) {
			buildToDelete = append(buildToDelete, b.Name)
			m, err := manifest.Load([]byte(b.Spec.Manifest), structs.Environment{})
			if err != nil {
				a.logger.Errorf("failed to load manifest for build '%s' of app '%s': %s", b.Name, app.Name, err)
				continue
			}
			for _, svc := range m.Services {
				if svc.Name != "" {
					buildTagsToDelete = append(buildTagsToDelete, fmt.Sprintf("%s.%s", svc.Name, strings.ToUpper(b.Name)))
				}
			}
		}
	}

	for _, b := range buildToDelete {
		a.convox.ConvoxV1().Builds(appNamespace).Delete(b, &am.DeleteOptions{})
		time.Sleep(50 * time.Millisecond) // to avoid rate limit
	}

	batchSize := 50
	for i := 0; i < len(buildTagsToDelete); i = i + batchSize {
		batch := buildTagsToDelete[i:min(i+batchSize, len(buildTagsToDelete))]
		if err := a.engine.RepositoryImagesBatchDelete(app.Name, batch); err != nil {
			a.logger.Errorf("failed to delete images for builds '%s': %s", strings.Join(batch, ","), err)
		}
	}
	return nil
}
