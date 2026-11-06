package k8s

import (
	"context"
	"fmt"

	v1 "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cinformer "github.com/convox/convox/provider/k8s/pkg/client/informers/externalversions"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func (p *Provider) RunSharedInformer(stopCh chan struct{}) {
	rcHandler := func(name string) cache.ResourceEventHandler {
		return cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) {},
			UpdateFunc: func(oldObj, newObj interface{}) {},
			DeleteFunc: func(obj interface{}) {},
		}
	}

	informerFactory := informers.NewSharedInformerFactory(p.Cluster, 0)
	namespaceInformer := informerFactory.Core().V1().Namespaces()
	nodeInformer := informerFactory.Core().V1().Nodes()
	podInformer := informerFactory.Core().V1().Pods()
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	namespaceInformer.Informer().AddEventHandler(rcHandler("namespace"))
	nodeInformer.Informer().AddEventHandler(rcHandler("node"))
	podInformer.Informer().AddEventHandler(rcHandler("pod"))
	deploymentInformer.Informer().AddEventHandler(rcHandler("deployment"))

	informerFactory.Start(stopCh)

	// Wait for caches to sync
	cache.WaitForCacheSync(stopCh, namespaceInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopCh, nodeInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopCh, podInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopCh, deploymentInformer.Informer().HasSynced)

	p.namespaceInformer = namespaceInformer
	p.nodeInformer = nodeInformer
	p.podInformer = podInformer
	p.deploymentInformer = deploymentInformer

	// convox custom informers
	convoxInformerFactory := cinformer.NewFilteredSharedInformerFactory(p.Convox, 0, am.NamespaceAll, func(opts *am.ListOptions) {
		opts.LabelSelector = "!convox.io/marked-as"
	})
	buildInformer := convoxInformerFactory.Convox().V1().Builds()
	releaseInformer := convoxInformerFactory.Convox().V1().Releases()

	buildInformer.Informer().AddEventHandler(rcHandler("build"))
	releaseInformer.Informer().AddEventHandler(rcHandler("release"))

	convoxInformerFactory.Start(stopCh)

	// Wait for caches to sync
	cache.WaitForCacheSync(stopCh, buildInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopCh, releaseInformer.Informer().HasSynced)

	p.buildInformer = buildInformer
	p.releaseInformer = releaseInformer

	p.logger.Logf("Informer started..\n")
}

func (p *Provider) GetNamespaceFromInformer(name string) (*corev1.Namespace, error) {
	if p.namespaceInformer == nil {
		return p.Cluster.CoreV1().Namespaces().Get(context.TODO(), name, am.GetOptions{})
	}
	ns, err := p.namespaceInformer.Lister().Get(name)
	if err != nil {
		return p.Cluster.CoreV1().Namespaces().Get(context.TODO(), name, am.GetOptions{})
	}
	p.logger.Logf("Namespace %s retrieved from informer\n", name)
	return ns, nil
}

func (p *Provider) ListNamespacesFromInformer(labelSelector string) (*corev1.NamespaceList, error) {
	if p.namespaceInformer == nil {
		return p.Cluster.CoreV1().Namespaces().List(context.TODO(), am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	var lbSelector labels.Selector
	var err error
	if labelSelector == "" {
		lbSelector = labels.Everything()
	} else {
		lbSelector, err = labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label selector: %w", err)
		}
	}

	nsList, err := p.namespaceInformer.Lister().List(lbSelector)
	if err != nil {
		return p.Cluster.CoreV1().Namespaces().List(context.TODO(), am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	resp := &corev1.NamespaceList{
		Items: make([]corev1.Namespace, len(nsList)),
	}
	for i, ns := range nsList {
		resp.Items[i] = *ns
	}
	p.logger.Logf("Namespaces retrieved from informer\n")
	return resp, nil
}

func (p *Provider) GetNodeFromInformer(name string) (*corev1.Node, error) {
	if p.nodeInformer == nil {
		return p.Cluster.CoreV1().Nodes().Get(context.TODO(), name, am.GetOptions{})
	}
	nd, err := p.nodeInformer.Lister().Get(name)
	if err != nil {
		return p.Cluster.CoreV1().Nodes().Get(context.TODO(), name, am.GetOptions{})
	}
	p.logger.Logf("Node %s retrieved from informer\n", name)
	return nd, nil
}

func (p *Provider) ListNodesFromInformer(labelSelector string) (*corev1.NodeList, error) {
	if p.nodeInformer == nil {
		return p.Cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	var lbSelector labels.Selector
	var err error
	if labelSelector == "" {
		lbSelector = labels.Everything()
	} else {
		lbSelector, err = labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label selector: %w", err)
		}
	}

	nsList, err := p.nodeInformer.Lister().List(lbSelector)
	if err != nil {
		return p.Cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	resp := &corev1.NodeList{
		Items: make([]corev1.Node, len(nsList)),
	}
	for i, ns := range nsList {
		resp.Items[i] = *ns
	}
	p.logger.Logf("Nodes retrieved from informer\n")
	return resp, nil
}

func (p *Provider) GetPodFromInformer(name, ns string) (*corev1.Pod, error) {
	if p.podInformer == nil {
		return p.Cluster.CoreV1().Pods(ns).Get(context.TODO(), name, am.GetOptions{})
	}
	pod, err := p.podInformer.Lister().Pods(ns).Get(name)
	if err != nil {
		return p.Cluster.CoreV1().Pods(ns).Get(context.TODO(), name, am.GetOptions{})
	}
	p.logger.Logf("Pod %s retrieved from informer\n", name)
	return pod, nil
}

func (p *Provider) ListPodsFromInformer(ns string, labelSelector string) (*corev1.PodList, error) {
	if p.podInformer == nil {
		return p.Cluster.CoreV1().Pods(ns).List(context.TODO(), am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	var lbSelector labels.Selector
	var err error
	if labelSelector == "" {
		lbSelector = labels.Everything()
	} else {
		lbSelector, err = labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label selector: %w", err)
		}
	}

	podList, err := p.podInformer.Lister().Pods(ns).List(lbSelector)
	if err != nil {
		return p.Cluster.CoreV1().Pods(ns).List(context.TODO(), am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	resp := &corev1.PodList{
		Items: make([]corev1.Pod, len(podList)),
	}
	for i, pod := range podList {
		resp.Items[i] = *pod
	}
	p.logger.Logf("Pods retrieved from informer\n")
	return resp, nil
}

func (p *Provider) GetDeploymentFromInformer(name, ns string) (*appsv1.Deployment, error) {
	if p.deploymentInformer == nil {
		return p.Cluster.AppsV1().Deployments(ns).Get(p.ctx, name, am.GetOptions{})
	}
	deployment, err := p.deploymentInformer.Lister().Deployments(ns).Get(name)
	if err != nil {
		return p.Cluster.AppsV1().Deployments(ns).Get(p.ctx, name, am.GetOptions{})
	}
	p.logger.Logf("Deployment %s retrieved from informer\n", name)
	return deployment, nil
}

func (p *Provider) ListDeploymentsFromInformer(ns string, labelSelector string) (*appsv1.DeploymentList, error) {
	if p.deploymentInformer == nil {
		return p.Cluster.AppsV1().Deployments(ns).List(p.ctx, am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	var lbSelector labels.Selector
	var err error
	if labelSelector == "" {
		lbSelector = labels.Everything()
	} else {
		lbSelector, err = labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label selector: %w", err)
		}
	}

	deploymentList, err := p.deploymentInformer.Lister().Deployments(ns).List(lbSelector)
	if err != nil {
		return p.Cluster.AppsV1().Deployments(ns).List(p.ctx, am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	resp := &appsv1.DeploymentList{
		Items: make([]appsv1.Deployment, len(deploymentList)),
	}
	for i, deployment := range deploymentList {
		resp.Items[i] = *deployment
	}
	p.logger.Logf("Deployments retrieved from informer\n")
	return resp, nil
}

func (p *Provider) GetBuildFromInformer(name, ns string) (*v1.Build, error) {
	if p.buildInformer == nil {
		return p.Convox.ConvoxV1().Builds(ns).Get(name, am.GetOptions{})
	}
	build, err := p.buildInformer.Lister().Builds(ns).Get(name)
	if err != nil {
		return p.Convox.ConvoxV1().Builds(ns).Get(name, am.GetOptions{})
	}
	p.logger.Logf("Build %s retrieved from informer\n", name)
	return build, nil
}

func (p *Provider) ListBuildsFromInformer(ns string, labelSelector string, limit int) (*v1.BuildList, error) {
	if p.buildInformer == nil || limit > 50 {
		return p.Convox.ConvoxV1().Builds(ns).List(am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	var lbSelector labels.Selector
	var err error
	if labelSelector == "" {
		lbSelector = labels.Everything()
	} else {
		lbSelector, err = labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label selector: %w", err)
		}
	}

	buildList, err := p.buildInformer.Lister().Builds(ns).List(lbSelector)
	if err != nil {
		return p.Convox.ConvoxV1().Builds(ns).List(am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	resp := &v1.BuildList{
		Items: make([]v1.Build, len(buildList)),
	}
	for i, build := range buildList {
		resp.Items[i] = *build
	}
	p.logger.Logf("Builds retrieved from informer\n")
	return resp, nil
}

func (p *Provider) GetReleaseFromInformer(name, ns string) (*v1.Release, error) {
	if p.releaseInformer == nil {
		return p.Convox.ConvoxV1().Releases(ns).Get(name, am.GetOptions{})
	}
	release, err := p.releaseInformer.Lister().Releases(ns).Get(name)
	if err != nil {
		return p.Convox.ConvoxV1().Releases(ns).Get(name, am.GetOptions{})
	}
	p.logger.Logf("Release %s retrieved from informer\n", name)
	return release, nil
}

func (p *Provider) ListReleasesFromInformer(ns string, labelSelector string, limit int) (*v1.ReleaseList, error) {
	if p.releaseInformer == nil || limit > 50 {
		return p.Convox.ConvoxV1().Releases(ns).List(am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	var lbSelector labels.Selector
	var err error
	if labelSelector == "" {
		lbSelector = labels.Everything()
	} else {
		lbSelector, err = labels.Parse(labelSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to parse label selector: %w", err)
		}
	}

	releaseList, err := p.releaseInformer.Lister().Releases(ns).List(lbSelector)
	if err != nil {
		return p.Convox.ConvoxV1().Releases(ns).List(am.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	resp := &v1.ReleaseList{
		Items: make([]v1.Release, len(releaseList)),
	}
	for i, release := range releaseList {
		resp.Items[i] = *release
	}
	p.logger.Logf("Releases retrieved from informer\n")
	return resp, nil
}
