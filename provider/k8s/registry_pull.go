package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	registryPullSecretName        = "convox-registry-authentication" //nolint:gosec // a Secret name, not a credential
	registryPullReconcileInterval = 10 * time.Minute
	registryPullTemplatePatch     = `{"spec":{"template":{"spec":{"imagePullSecrets":%s}}}}`
	registryPullCronJobPatch      = `{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"imagePullSecrets":%s}}}}}}`
)

func (p *Provider) hasRegistryPullCredentials() bool {
	_, ok := p.Engine.(RegistryPullEngine)
	return ok
}

func (p *Provider) ensureRegistryPullSecret(ctx context.Context, namespace string) error {
	e, ok := p.Engine.(RegistryPullEngine)
	if !ok {
		return nil
	}

	data, err := buildDockerConfigJSON(e.RegistryPullCredentials())
	if err != nil {
		return errors.WithStack(err)
	}

	_, err = p.CreateOrPatchSecret(ctx, am.ObjectMeta{
		Name:      registryPullSecretName,
		Namespace: namespace,
	}, func(s *ac.Secret) *ac.Secret {
		s.Labels = map[string]string{"system": "convox", "rack": p.Name}
		s.Type = ac.SecretTypeDockerConfigJson
		s.Data = map[string][]byte{ac.DockerConfigJsonKey: data}
		return s
	}, am.PatchOptions{
		FieldManager: "convox",
	})
	if k8serrors.IsAlreadyExists(err) {
		return nil
	}

	return errors.WithStack(err)
}

func (p *Provider) reconcileRegistryPull(ctx context.Context) error {
	e, ok := p.Engine.(RegistryPullEngine)
	if !ok {
		return nil
	}

	server, _, _ := e.RegistryPullCredentials()

	nss, err := p.ListNamespacesFromInformer(fmt.Sprintf("system=convox,rack=%s,type=app", p.Name))
	if err != nil {
		return errors.WithStack(err)
	}

	for i := range nss.Items {
		ns := nss.Items[i].Name

		if rerr := p.reconcileRegistryPullNamespace(ctx, ns, server+"/"); rerr != nil {
			fmt.Printf("ns=registry_pull at=warn kind=reconcile rack=%s namespace=%s err=%q\n", p.Name, ns, rerr)
		}
	}

	return nil
}

func (p *Provider) reconcileRegistryPullNamespace(ctx context.Context, ns, prefix string) error {
	if err := p.ensureRegistryPullSecret(ctx, ns); err != nil {
		return err
	}

	sas, err := p.Cluster.CoreV1().ServiceAccounts(ns).List(ctx, am.ListOptions{})
	if err != nil {
		return err
	}

	for i := range sas.Items {
		sa := &sas.Items[i]
		if hasRegistryPullSecret(sa.ImagePullSecrets) {
			continue
		}

		sa.ImagePullSecrets = append(sa.ImagePullSecrets, ac.LocalObjectReference{Name: registryPullSecretName})

		if _, err := p.Cluster.CoreV1().ServiceAccounts(ns).Update(ctx, sa, am.UpdateOptions{FieldManager: "convox"}); err != nil && !k8serrors.IsConflict(err) {
			return err
		}
	}

	if err := p.patchRegistryPullWorkloads(ctx, ns, prefix); err != nil {
		return err
	}

	return p.deleteRegistryPullStuckPods(ctx, ns, prefix)
}

func (p *Provider) patchRegistryPullWorkloads(ctx context.Context, ns, prefix string) error {
	opts := am.PatchOptions{FieldManager: "convox"}

	ds, err := p.Cluster.AppsV1().Deployments(ns).List(ctx, am.ListOptions{})
	if err != nil {
		return err
	}
	for i := range ds.Items {
		if err := patchRegistryPull(&ds.Items[i].Spec.Template.Spec, prefix, registryPullTemplatePatch, func(patch []byte) error {
			_, err := p.Cluster.AppsV1().Deployments(ns).Patch(ctx, ds.Items[i].Name, types.StrategicMergePatchType, patch, opts)
			return err
		}); err != nil {
			return err
		}
	}

	sts, err := p.Cluster.AppsV1().StatefulSets(ns).List(ctx, am.ListOptions{})
	if err != nil {
		return err
	}
	for i := range sts.Items {
		if err := patchRegistryPull(&sts.Items[i].Spec.Template.Spec, prefix, registryPullTemplatePatch, func(patch []byte) error {
			_, err := p.Cluster.AppsV1().StatefulSets(ns).Patch(ctx, sts.Items[i].Name, types.StrategicMergePatchType, patch, opts)
			return err
		}); err != nil {
			return err
		}
	}

	dss, err := p.Cluster.AppsV1().DaemonSets(ns).List(ctx, am.ListOptions{})
	if err != nil {
		return err
	}
	for i := range dss.Items {
		if err := patchRegistryPull(&dss.Items[i].Spec.Template.Spec, prefix, registryPullTemplatePatch, func(patch []byte) error {
			_, err := p.Cluster.AppsV1().DaemonSets(ns).Patch(ctx, dss.Items[i].Name, types.StrategicMergePatchType, patch, opts)
			return err
		}); err != nil {
			return err
		}
	}

	cjs, err := p.Cluster.BatchV1().CronJobs(ns).List(ctx, am.ListOptions{})
	if err != nil {
		return err
	}
	for i := range cjs.Items {
		if err := patchRegistryPull(&cjs.Items[i].Spec.JobTemplate.Spec.Template.Spec, prefix, registryPullCronJobPatch, func(patch []byte) error {
			_, err := p.Cluster.BatchV1().CronJobs(ns).Patch(ctx, cjs.Items[i].Name, types.StrategicMergePatchType, patch, opts)
			return err
		}); err != nil {
			return err
		}
	}

	return nil
}

// The patch carries the whole list: a merge patch of only the new entry would put it first, and the
// template renders it last, so the next promote would roll the workload again to reorder it.
func patchRegistryPull(spec *ac.PodSpec, prefix, format string, apply func([]byte) error) error {
	if !needsRegistryPullSecret(spec, prefix) {
		return nil
	}

	refs, err := json.Marshal(append(slices.Clone(spec.ImagePullSecrets), ac.LocalObjectReference{Name: registryPullSecretName}))
	if err != nil {
		return err
	}

	return apply(fmt.Appendf(nil, format, refs))
}

func (p *Provider) deleteRegistryPullStuckPods(ctx context.Context, ns, prefix string) error {
	pods, err := p.Cluster.CoreV1().Pods(ns).List(ctx, am.ListOptions{})
	if err != nil {
		return err
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.DeletionTimestamp != nil || hasRegistryPullSecret(pod.Spec.ImagePullSecrets) || !pullStuckOnRegistry(pod, prefix) {
			continue
		}

		if err := p.Cluster.CoreV1().Pods(ns).Delete(ctx, pod.Name, am.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func hasRegistryPullSecret(refs []ac.LocalObjectReference) bool {
	return slices.ContainsFunc(refs, func(r ac.LocalObjectReference) bool { return r.Name == registryPullSecretName })
}

func needsRegistryPullSecret(spec *ac.PodSpec, prefix string) bool {
	if len(spec.ImagePullSecrets) == 0 || hasRegistryPullSecret(spec.ImagePullSecrets) {
		return false
	}

	return slices.ContainsFunc(slices.Concat(spec.InitContainers, spec.Containers), func(c ac.Container) bool {
		return strings.HasPrefix(c.Image, prefix)
	})
}

func pullStuckOnRegistry(pod *ac.Pod, prefix string) bool {
	containers := slices.Concat(pod.Spec.InitContainers, pod.Spec.Containers)
	images := make(map[string]string, len(containers))
	for i := range containers {
		images[containers[i].Name] = containers[i].Image
	}

	return slices.ContainsFunc(slices.Concat(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses), func(cs ac.ContainerStatus) bool {
		w := cs.State.Waiting
		return w != nil && (w.Reason == "ErrImagePull" || w.Reason == "ImagePullBackOff") && strings.HasPrefix(images[cs.Name], prefix)
	})
}

func (p *Provider) reconcileRegistryPullSafe(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=registry_pull at=error kind=panic_recovered rack=%s recovered=%v\n", p.Name, r)
		}
	}()

	if err := p.reconcileRegistryPull(ctx); err != nil {
		fmt.Printf("ns=registry_pull at=warn kind=reconcile rack=%s err=%q\n", p.Name, err)
	}
}

func (p *Provider) runRegistryPullReconciler(ctx context.Context) {
	if !p.hasRegistryPullCredentials() {
		return
	}

	p.reconcileRegistryPullSafe(ctx)

	tick := time.NewTicker(registryPullReconcileInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			p.reconcileRegistryPullSafe(ctx)
		}
	}
}
