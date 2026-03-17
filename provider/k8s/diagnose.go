package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var diagnosticHints = map[string]string{
	"CrashLoopBackOff":          "Process is crash-looping on startup -- check the logs below for the error",
	"ImagePullBackOff":          "Failed to pull the container image -- check that the build succeeded and the image tag exists",
	"ErrImagePull":              "Failed to pull the container image -- check registry access and image name",
	"CreateContainerConfigError": "Container config is invalid -- check environment variables and secrets (missing env var or secret reference?)",
	"RunContainerError":         "Container failed to start -- check the command in convox.yml and that the entrypoint exists",
	"OOMKilled":                 "Process ran out of memory and was killed -- increase scale.memory in convox.yml",
	"Completed":                 "Process exited successfully but is not expected to stop -- check your command does not exit on its own",
	"Error":                     "Process exited with an error -- check the logs below",
	"ContainerCannotRun":        "Container cannot run -- check that the Dockerfile CMD or convox.yml command is valid",
	"InvalidImageName":          "Image name is invalid -- check build configuration",
	"StartError":                "Container failed to start -- check logs below for the startup error",
	"Unschedulable":             "Not enough resources in the cluster to place this process -- check scale.cpu and scale.memory in convox.yml",
	"ContainersNotReady":        "Containers are not ready -- health check may be failing",
	"PodInitializing":           "Pod is still initializing -- init containers may still be running",
}

var eventHints = map[string]string{
	"FailedCreate":              "Could not create new processes",
	"FailedScheduling":          "Could not place process -- not enough capacity in the cluster",
	"ImagePullBackOff":          "Failed to pull the service image -- check build output and registry access",
	"ErrImagePull":              "Failed to pull the service image -- check build output and registry access",
	"ProgressDeadlineExceeded":  "Deploy timed out -- processes did not become healthy in time",
	"OOMKilled":                 "Process ran out of memory -- increase scale.memory in convox.yml",
	"FailedMount":               "Failed to mount volume -- check volumeOptions in convox.yml",
	"FailedAttachVolume":        "Failed to attach volume",
	"Evicted":                   "Process was evicted (cluster resource pressure)",
	"InsufficientCPU":           "Not enough CPU in the cluster -- check scale.cpu in convox.yml",
	"InsufficientMemory":        "Not enough memory in the cluster -- check scale.memory in convox.yml",
	"BackOff":                   "Container is in back-off -- restarting failed container",
	"Unhealthy":                 "Health check is failing -- check health.path in convox.yml",
}

func getHint(stateDetail string) string {
	if hint, ok := diagnosticHints[stateDetail]; ok {
		return hint
	}
	for key, hint := range diagnosticHints {
		if strings.HasSuffix(stateDetail, key) || strings.Contains(stateDetail, key) {
			return hint
		}
	}
	if strings.Contains(stateDetail, "Pending") {
		return "Process is waiting to be scheduled -- this usually means the cluster is low on resources"
	}
	return ""
}

func getEventHint(reason string) string {
	if hint, ok := eventHints[reason]; ok {
		return hint
	}
	return ""
}

func (p *Provider) AppDiagnose(app string, opts structs.AppDiagnoseOptions) (*structs.AppDiagnosticReport, error) {
	namespace := p.AppNamespace(app)

	_, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), namespace, am.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("namespace %s not found -- verify rack and app names", namespace)
	}

	report := &structs.AppDiagnosticReport{
		Namespace: namespace,
		Rack:      p.Name,
		App:       app,
		Timestamp: time.Now().UTC(),
	}

	checks := []string{"overview", "init", "services"}
	if opts.Checks != nil && *opts.Checks != "" {
		checks = strings.Split(*opts.Checks, ",")
	}

	for _, check := range checks {
		switch strings.TrimSpace(check) {
		case "overview":
			overview, err := p.diagnoseOverview(namespace, opts)
			if err != nil {
				return nil, err
			}
			report.Overview = overview
		case "init":
			initPods, err := p.diagnoseInitPods(namespace)
			if err != nil {
				return nil, err
			}
			report.InitPods = initPods
		case "services":
			pods, summary, err := p.diagnosePods(namespace, opts)
			if err != nil {
				return nil, err
			}
			report.Pods = pods
			report.Summary = summary
		}
	}

	return report, nil
}

func (p *Provider) diagnoseOverview(namespace string, opts structs.AppDiagnoseOptions) (*structs.DiagnosticOverview, error) {
	overview := &structs.DiagnosticOverview{}

	serviceFilter := map[string]bool{}
	if opts.Services != nil && *opts.Services != "" {
		for _, s := range strings.Split(*opts.Services, ",") {
			serviceFilter[strings.TrimSpace(s)] = true
		}
	}

	// List Deployments
	ds, err := p.Cluster.AppsV1().Deployments(namespace).List(context.TODO(), am.ListOptions{
		LabelSelector: "system=convox,type=service",
	})
	if err != nil {
		return nil, err
	}

	for _, d := range ds.Items {
		name := d.ObjectMeta.Name
		if len(serviceFilter) > 0 && !serviceFilter[name] {
			continue
		}

		desired := int(common.DefaultInt32(d.Spec.Replicas, 1))
		ready := int(d.Status.ReadyReplicas)
		updated := int(d.Status.UpdatedReplicas)

		status := "running"
		stallReason := ""

		if desired == 0 {
			status = "scaled-down"
		} else if ready == desired && updated == desired {
			status = "running"
		} else {
			status = "deploying"
			for _, cond := range d.Status.Conditions {
				if cond.Type == "Progressing" && cond.Reason == "ProgressDeadlineExceeded" {
					status = "stalled"
					stallReason = "Deploy timed out -- processes did not become healthy in time"
					break
				}
			}
		}

		overview.Services = append(overview.Services, structs.DiagnosticServiceStatus{
			Name:            name,
			DesiredReplicas: desired,
			ReadyReplicas:   ready,
			UpdatedReplicas: updated,
			Status:          status,
			StallReason:     stallReason,
		})
	}

	// List DaemonSets
	dss, err := p.Cluster.AppsV1().DaemonSets(namespace).List(context.TODO(), am.ListOptions{
		LabelSelector: "system=convox,type=service",
	})
	if err != nil {
		return nil, err
	}

	for _, d := range dss.Items {
		name := d.ObjectMeta.Name
		if len(serviceFilter) > 0 && !serviceFilter[name] {
			continue
		}

		desired := int(d.Status.DesiredNumberScheduled)
		ready := int(d.Status.NumberReady)

		status := "running"
		if ready < desired {
			status = "deploying"
		}

		overview.Services = append(overview.Services, structs.DiagnosticServiceStatus{
			Name:            name,
			DesiredReplicas: desired,
			ReadyReplicas:   ready,
			UpdatedReplicas: int(d.Status.UpdatedNumberScheduled),
			Status:          status,
			Agent:           true,
		})
	}

	// Sort services by name
	sort.Slice(overview.Services, func(i, j int) bool {
		return overview.Services[i].Name < overview.Services[j].Name
	})

	// List Warning Events (last 30 minutes)
	events, err := p.Cluster.CoreV1().Events(namespace).List(context.TODO(), am.ListOptions{})
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-30 * time.Minute)
	for _, ev := range events.Items {
		evTime := ev.LastTimestamp.Time
		if evTime.IsZero() {
			evTime = ev.CreationTimestamp.Time
		}
		if evTime.Before(cutoff) {
			continue
		}
		if ev.Type != "Warning" {
			continue
		}

		overview.Events = append(overview.Events, structs.DiagnosticEvent{
			Timestamp: evTime,
			Type:      ev.Type,
			Reason:    ev.Reason,
			Object:    fmt.Sprintf("%s/%s", strings.ToLower(ev.InvolvedObject.Kind), ev.InvolvedObject.Name),
			Message:   ev.Message,
			Hint:      getEventHint(ev.Reason),
		})
	}

	// Sort events by timestamp descending
	sort.Slice(overview.Events, func(i, j int) bool {
		return overview.Events[i].Timestamp.After(overview.Events[j].Timestamp)
	})

	return overview, nil
}

func (p *Provider) diagnoseInitPods(namespace string) ([]structs.DiagnosticInitPod, error) {
	pods, err := p.Cluster.CoreV1().Pods(namespace).List(context.TODO(), am.ListOptions{
		LabelSelector: "system=convox",
	})
	if err != nil {
		return nil, err
	}

	var initPods []structs.DiagnosticInitPod

	for _, pod := range pods.Items {
		hasStuckInit := false
		for _, ics := range pod.Status.InitContainerStatuses {
			if !ics.Ready {
				hasStuckInit = true
				break
			}
		}
		if !hasStuckInit {
			continue
		}

		ip := structs.DiagnosticInitPod{
			Name:    pod.Name,
			Service: pod.Labels["service"],
			Phase:   string(pod.Status.Phase),
		}

		for _, ics := range pod.Status.InitContainerStatuses {
			state := "unknown"
			if ics.State.Running != nil {
				state = "Running"
			} else if ics.State.Waiting != nil {
				state = ics.State.Waiting.Reason
				if state == "" {
					state = "Waiting"
				}
			} else if ics.State.Terminated != nil {
				state = fmt.Sprintf("Terminated:%s(exit=%d)", ics.State.Terminated.Reason, ics.State.Terminated.ExitCode)
			}

			dc := structs.DiagnosticContainer{
				Name:  ics.Name,
				State: state,
			}

			// Get init container logs
			logs, err := p.getPodLogs(namespace, pod.Name, ics.Name, 100)
			if err == nil {
				dc.Logs = logs
			}

			ip.InitContainers = append(ip.InitContainers, dc)
		}

		initPods = append(initPods, ip)
	}

	return initPods, nil
}

func classifyPod(pod ac.Pod, ageThreshold int) (classification, stateDetail string) {
	phase := string(pod.Status.Phase)
	age := int(time.Since(pod.CreationTimestamp.Time).Seconds())

	ready := true
	if len(pod.Status.ContainerStatuses) == 0 {
		ready = false
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			ready = false
			break
		}
	}

	// Extract state detail from first non-ready container
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			stateDetail = cs.State.Waiting.Reason
			break
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
			stateDetail = fmt.Sprintf("Terminated:%s", cs.State.Terminated.Reason)
			break
		}
	}

	// Check init container statuses
	if stateDetail == "" {
		for _, ics := range pod.Status.InitContainerStatuses {
			if ics.State.Waiting != nil && ics.State.Waiting.Reason != "" {
				stateDetail = fmt.Sprintf("init:%s", ics.State.Waiting.Reason)
				break
			}
			if ics.State.Terminated != nil && ics.State.Terminated.ExitCode != 0 {
				stateDetail = fmt.Sprintf("init:Failed(exit=%d)", ics.State.Terminated.ExitCode)
				break
			}
		}
	}

	switch {
	case phase != "Running":
		classification = "unhealthy"
	case !ready:
		classification = "not-ready"
	case age <= ageThreshold:
		classification = "new"
	default:
		classification = "healthy"
	}

	return
}

func (p *Provider) diagnosePods(namespace string, opts structs.AppDiagnoseOptions) ([]structs.DiagnosticPod, *structs.DiagnosticSummary, error) {
	pods, err := p.Cluster.CoreV1().Pods(namespace).List(context.TODO(), am.ListOptions{
		LabelSelector: "system=convox",
	})
	if err != nil {
		return nil, nil, err
	}

	ageThreshold := 300
	if opts.AgeThreshold != nil {
		ageThreshold = *opts.AgeThreshold
	}

	lines := int64(200)
	if opts.Lines != nil {
		lines = int64(*opts.Lines)
	}

	showAll := opts.All != nil && *opts.All
	includeEvents := opts.Events == nil || *opts.Events
	includePrevious := opts.Previous == nil || *opts.Previous

	serviceFilter := map[string]bool{}
	if opts.Services != nil && *opts.Services != "" {
		for _, s := range strings.Split(*opts.Services, ",") {
			serviceFilter[strings.TrimSpace(s)] = true
		}
	}

	summary := &structs.DiagnosticSummary{}

	// First pass: classify all pods
	type classifiedPod struct {
		pod            ac.Pod
		classification string
		stateDetail    string
	}

	var classified []classifiedPod
	for _, pod := range pods.Items {
		service := pod.Labels["service"]
		if len(serviceFilter) > 0 && !serviceFilter[service] {
			continue
		}

		classification, stateDetail := classifyPod(pod, ageThreshold)

		switch classification {
		case "unhealthy":
			summary.Unhealthy++
		case "not-ready":
			summary.NotReady++
		case "new":
			summary.New++
		case "healthy":
			summary.Healthy++
		}
		summary.Total++

		if !showAll && classification == "healthy" {
			continue
		}

		classified = append(classified, classifiedPod{
			pod:            pod,
			classification: classification,
			stateDetail:    stateDetail,
		})
	}

	// Sort: unhealthy first, then not-ready, then new, then healthy
	classOrder := map[string]int{"unhealthy": 0, "not-ready": 1, "new": 2, "healthy": 3}
	sort.Slice(classified, func(i, j int) bool {
		ci := classOrder[classified[i].classification]
		cj := classOrder[classified[j].classification]
		if ci != cj {
			return ci < cj
		}
		return classified[i].pod.Name < classified[j].pod.Name
	})

	// Second pass: collect logs and events concurrently with semaphore
	var result []structs.DiagnosticPod
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // max 5 concurrent log fetches

	for _, cp := range classified {
		wg.Add(1)
		go func(cp classifiedPod) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			readyCount := 0
			totalContainers := len(cp.pod.Status.ContainerStatuses)
			for _, cs := range cp.pod.Status.ContainerStatuses {
				if cs.Ready {
					readyCount++
				}
			}
			if totalContainers == 0 {
				totalContainers = len(cp.pod.Spec.Containers)
			}

			restarts := 0
			for _, cs := range cp.pod.Status.ContainerStatuses {
				restarts += int(cs.RestartCount)
			}

			dp := structs.DiagnosticPod{
				Name:           cp.pod.Name,
				Service:        cp.pod.Labels["service"],
				Phase:          string(cp.pod.Status.Phase),
				Ready:          fmt.Sprintf("%d/%d", readyCount, totalContainers),
				AgeSeconds:     int(time.Since(cp.pod.CreationTimestamp.Time).Seconds()),
				Restarts:       restarts,
				Classification: cp.classification,
				StateDetail:    cp.stateDetail,
				Hint:           getHint(cp.stateDetail),
			}

			// Add pending phase hint if no other hint
			if dp.Hint == "" && cp.pod.Status.Phase == ac.PodPending {
				dp.Hint = "Process is waiting to be scheduled -- this usually means the cluster is low on resources"
			}

			// Get current logs
			currentLogs, err := p.getAllContainerLogs(namespace, cp.pod.Name, lines)
			if err == nil {
				dp.Logs = currentLogs
			}

			// Get previous container crash logs
			if includePrevious {
				prevLogs, err := p.getPreviousLogs(namespace, cp.pod, lines)
				if err == nil && prevLogs != "" {
					dp.PreviousLogs = prevLogs
				}
			}

			// Get events for this pod
			if includeEvents {
				events, err := p.Cluster.CoreV1().Events(namespace).List(context.TODO(), am.ListOptions{
					FieldSelector: fmt.Sprintf("involvedObject.name=%s", cp.pod.Name),
				})
				if err == nil {
					for _, ev := range events.Items {
						evTime := ev.LastTimestamp.Time
						if evTime.IsZero() {
							evTime = ev.CreationTimestamp.Time
						}
						dp.Events = append(dp.Events, structs.DiagnosticEvent{
							Timestamp: evTime,
							Type:      ev.Type,
							Reason:    ev.Reason,
							Object:    fmt.Sprintf("%s/%s", strings.ToLower(ev.InvolvedObject.Kind), ev.InvolvedObject.Name),
							Message:   ev.Message,
							Hint:      getEventHint(ev.Reason),
						})
					}
					sort.Slice(dp.Events, func(i, j int) bool {
						return dp.Events[i].Timestamp.After(dp.Events[j].Timestamp)
					})
				}
			}

			mu.Lock()
			result = append(result, dp)
			mu.Unlock()
		}(cp)
	}

	wg.Wait()

	// Re-sort after concurrent collection
	sort.Slice(result, func(i, j int) bool {
		ci := classOrder[result[i].Classification]
		cj := classOrder[result[j].Classification]
		if ci != cj {
			return ci < cj
		}
		return result[i].Name < result[j].Name
	})

	return result, summary, nil
}

func (p *Provider) getPodLogs(namespace, podName, container string, lines int64) (string, error) {
	opts := &ac.PodLogOptions{
		Container: container,
		TailLines: &lines,
	}

	req := p.Cluster.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer stream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (p *Provider) getAllContainerLogs(namespace, podName string, lines int64) (string, error) {
	opts := &ac.PodLogOptions{
		TailLines: &lines,
	}

	// Try with all-containers first (requires iterating containers)
	pod, err := p.Cluster.CoreV1().Pods(namespace).Get(context.TODO(), podName, am.GetOptions{})
	if err != nil {
		return "", err
	}

	var allLogs strings.Builder
	containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)

	for _, c := range containers {
		opts.Container = c.Name
		req := p.Cluster.CoreV1().Pods(namespace).GetLogs(podName, opts)
		stream, err := req.Stream(context.TODO())
		if err != nil {
			continue
		}

		var buf bytes.Buffer
		io.Copy(&buf, stream)
		stream.Close()

		if buf.Len() > 0 {
			if len(containers) > 1 {
				allLogs.WriteString(fmt.Sprintf("==> container/%s <==\n", c.Name))
			}
			allLogs.WriteString(buf.String())
			if !strings.HasSuffix(buf.String(), "\n") {
				allLogs.WriteString("\n")
			}
		}
	}

	return allLogs.String(), nil
}

func (p *Provider) getPreviousLogs(namespace string, pod ac.Pod, lines int64) (string, error) {
	var allLogs strings.Builder

	for _, c := range pod.Spec.Containers {
		opts := &ac.PodLogOptions{
			Container: c.Name,
			Previous:  true,
			TailLines: &lines,
		}

		req := p.Cluster.CoreV1().Pods(namespace).GetLogs(pod.Name, opts)
		stream, err := req.Stream(context.TODO())
		if err != nil {
			continue // no previous container
		}

		var buf bytes.Buffer
		io.Copy(&buf, stream)
		stream.Close()

		if buf.Len() > 0 {
			if len(pod.Spec.Containers) > 1 {
				allLogs.WriteString(fmt.Sprintf("==> container/%s <==\n", c.Name))
			}
			allLogs.WriteString(buf.String())
			if !strings.HasSuffix(buf.String(), "\n") {
				allLogs.WriteString("\n")
			}
		}
	}

	return allLogs.String(), nil
}

