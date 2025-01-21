package k8s

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func (p *Provider) ProcessExec(app, pid, command string, rw io.ReadWriter, opts structs.ProcessExecOptions) (int, error) {
	pss, err := p.ProcessList(app, structs.ProcessListOptions{Service: options.String(pid)})
	if err != nil {
		return 0, err
	}

	// if pid is a service name, pick one at random
	if len(pss) > 0 {
		pid = pss[rand.Intn(len(pss))].Id
	}

	req := p.Cluster.CoreV1().RESTClient().Post().Resource("pods").Name(pid).Namespace(p.AppNamespace(app)).SubResource("exec").Param("container", app)

	cp, err := shellquote.Split(command)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	if common.DefaultBool(opts.Entrypoint, false) {
		ps, err := p.ProcessGet(app, pid)
		if err != nil {
			return 0, errors.WithStack(err)
		}

		r, err := p.ReleaseGet(app, ps.Release)
		if err != nil {
			return 0, errors.WithStack(err)
		}

		b, err := p.BuildGet(app, r.Build)
		if err != nil {
			return 0, errors.WithStack(err)
		}

		if b.Entrypoint != "" {
			ep, err := shellquote.Split(b.Entrypoint)
			if err != nil {
				return 0, errors.WithStack(err)
			}

			cp = append(ep, cp...)
		}
	}

	eo := &ac.PodExecOptions{
		Container: app,
		Command:   cp,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}

	if opts.DisableStdin != nil && *opts.DisableStdin {
		eo.Stdin = false
	}

	if opts.Tty != nil && !*opts.Tty {
		eo.TTY = false
	}

	req.VersionedParams(eo, scheme.ParameterCodec)

	e, err := remotecommand.NewSPDYExecutor(p.Config, "POST", req.URL())
	if err != nil {
		return 0, errors.WithStack(err)
	}

	sopts := remotecommand.StreamOptions{
		Stdout: rw,
		Stderr: rw,
		Tty:    true,
	}

	if !eo.Stdin && !eo.TTY {
		sopts.Tty = false
	} else {
		if eo.Stdin {
			inr, inw := io.Pipe()
			go io.Copy(inw, rw)

			sopts.Stdin = inr
		}

		if opts.Height != nil && opts.Width != nil {
			sopts.TerminalSizeQueue = &terminalSize{Height: *opts.Height, Width: *opts.Width}
		}

	}

	err = e.StreamWithContext(p.ctx, sopts)
	if ee, ok := err.(exec.ExitError); ok {
		return ee.ExitStatus(), nil
	}
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return 0, nil
}

func (p *Provider) ProcessGet(app, pid string) (*structs.Process, error) {
	pd, err := p.Cluster.CoreV1().Pods(p.AppNamespace(app)).Get(context.TODO(), pid, am.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ps, err := p.processFromPod(*pd)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	m, err := p.MetricsClient.MetricsV1beta1().PodMetricses(p.AppNamespace(app)).Get(context.TODO(), pid, am.GetOptions{})
	if err != nil {
		p.logger.Errorf("failed to fetch pod metrics: %s", err)
	} else if m != nil && len(m.Containers) > 0 {
		ps.Cpu, ps.Memory = calculatePodCpuAndMem(m)
	}

	return ps, nil
}

func (p *Provider) ProcessList(app string, opts structs.ProcessListOptions) (structs.Processes, error) {
	filters := []string{
		"system=convox",
		"type in (process,service,timer)",
	}

	if opts.Release != nil {
		filters = append(filters, fmt.Sprintf("release=%s", *opts.Release))
	}

	if opts.Service != nil {
		filters = append(filters, fmt.Sprintf("service=%s", *opts.Service))
	}

	pds, err := p.Cluster.CoreV1().Pods(p.AppNamespace(app)).List(context.TODO(), am.ListOptions{LabelSelector: strings.Join(filters, ",")})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	pss := structs.Processes{}

	for _, pd := range pds.Items {
		ps, err := p.processFromPod(pd)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		pss = append(pss, *ps)
	}

	ms, err := p.MetricsClient.MetricsV1beta1().PodMetricses(p.AppNamespace(app)).List(context.TODO(), am.ListOptions{LabelSelector: strings.Join(filters, ",")})
	if err != nil {
		p.logger.Errorf("failed to fetch pod metrics: %s", err)
	} else {
		metricsByPod := map[string]metricsv1beta1.PodMetrics{}
		for _, m := range ms.Items {
			metricsByPod[m.Name] = m
		}

		for i := range pss {
			if m, has := metricsByPod[pss[i].Id]; has && len(m.Containers) > 0 {
				pss[i].Cpu, pss[i].Memory = calculatePodCpuAndMem(&m)
			}
		}
	}

	return pss, nil
}

func (p *Provider) ProcessLogs(app, pid string, opts structs.LogsOptions) (io.ReadCloser, error) {
	r, w := io.Pipe()

	go p.streamProcessLogs(w, app, pid, opts)

	return r, nil
}

func (p *Provider) streamProcessLogs(w io.WriteCloser, app, pid string, opts structs.LogsOptions) {
	defer w.Close() // skipcq

	lopts := &ac.PodLogOptions{
		Follow:     true,
		Timestamps: true,
	}

	if opts.Since != nil {
		since := am.NewTime(time.Now().UTC().Add(*opts.Since))
		lopts.SinceTime = &since
	}

	service := ""

	for {
		pp, err := p.Cluster.CoreV1().Pods(p.AppNamespace(app)).Get(context.TODO(), pid, am.GetOptions{})
		if err != nil {
			fmt.Printf("err: %+v\n", err)
			break
		}

		service = pp.Labels["service"]

		if pp.Status.Phase != "Pending" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	for {
		r, err := p.Cluster.CoreV1().Pods(p.AppNamespace(app)).GetLogs(pid, lopts).Stream(context.TODO())
		if err != nil {
			fmt.Printf("err: %+v\n", err)
			break
		}

		s := bufio.NewScanner(r)

		s.Buffer(make([]byte, ScannerStartSize), ScannerMaxSize)

		for s.Scan() {
			line := s.Text()

			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				fmt.Printf("err: short line\n")
				continue
			}

			ts, err := time.Parse(time.RFC3339Nano, parts[0])
			if err != nil {
				fmt.Printf("err: %+v\n", err)
				continue
			}

			prefix := ""

			since := am.NewTime(ts)
			lopts.SinceTime = &since

			if common.DefaultBool(opts.Prefix, false) {
				prefix = fmt.Sprintf("%s %s ", ts.Format(time.RFC3339), fmt.Sprintf("service/%s/%s", service, pid))
			}

			fmt.Fprintf(w, "%s%s\n", prefix, strings.TrimSuffix(parts[1], "\n"))
		}

		if err := s.Err(); err != nil {
			fmt.Printf("err: %+v\n", err)
			continue
		}

		return
	}
}

func (p *Provider) ProcessRun(app, service string, opts structs.ProcessRunOptions) (*structs.Process, error) {
	s, err := p.podSpecFromRunOptions(app, service, opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	release := common.DefaultString(opts.Release, "")

	if release == "" {
		a, err := p.AppGet(app)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		release = a.Release
	}

	ans := map[string]string{}
	if service == "build" {
		for idx := 0; idx < len(s.Containers); idx++ {
			// assign the build container a BestEffor QoS to avoid eating all compute resources
			s.Containers[idx].Resources = ac.ResourceRequirements{}
			s.Containers[idx].SecurityContext = &ac.SecurityContext{SeccompProfile: &ac.SeccompProfile{Type: ac.SeccompProfileTypeUnconfined}}
			if opts.Privileged != nil && *opts.Privileged {
				s.Containers[idx].SecurityContext.Privileged = options.Bool(true)
				ans[fmt.Sprintf("container.apparmor.security.beta.kubernetes.io/%s", s.Containers[idx].Name)] = "unconfined"
			}
		}
		s.RestartPolicy = ac.RestartPolicyNever
		if p.BuildNodeEnabled == "true" {
			s.Affinity = &ac.Affinity{
				NodeAffinity: &ac.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &ac.NodeSelector{
						NodeSelectorTerms: []ac.NodeSelectorTerm{
							{
								MatchExpressions: []ac.NodeSelectorRequirement{
									{
										Key:      "convox-build",
										Operator: ac.NodeSelectorOpIn,
										Values: []string{
											"true",
										},
									},
								},
							},
						},
					},
				},
			}
			s.Tolerations = []ac.Toleration{
				{
					Key:      "dedicated",
					Operator: ac.TolerationOpExists,
					Effect:   ac.TaintEffectNoSchedule,
				},
			}
		}
	}

	pod := &ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Annotations:  ans,
			GenerateName: fmt.Sprintf("%s-", service),
			Labels: map[string]string{
				"app":     app,
				"rack":    p.Name,
				"release": release,
				"service": service,
				"system":  "convox",
				"type":    "process",
				"name":    service,
			},
		},
		Spec: *s,
	}

	pd, err := p.Cluster.CoreV1().Pods(p.AppNamespace(app)).Create(
		context.TODO(),
		pod,
		am.CreateOptions{},
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ps, err := p.ProcessGet(app, pd.ObjectMeta.Name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return ps, nil
}

func (p *Provider) ProcessStop(app, pid string) error {
	if err := p.Cluster.CoreV1().Pods(p.AppNamespace(app)).Delete(context.TODO(), pid, am.DeleteOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) ProcessWait(app, pid string) (int, error) {
	for {
		pd, err := p.Cluster.CoreV1().Pods(p.AppNamespace(app)).Get(context.TODO(), pid, am.GetOptions{})
		if err != nil {
			return 0, errors.WithStack(err)
		}

		cs := pd.Status.ContainerStatuses

		if len(cs) != 1 || cs[0].Name != app {
			return 0, errors.WithStack(fmt.Errorf("unexpected containers for pid: %s", pid))
		}

		if t := cs[0].State.Terminated; t != nil {
			if err := p.ProcessStop(app, pid); err != nil {
				return 0, errors.WithStack(err)
			}

			return int(t.ExitCode), nil
		}
	}
}

func (p *Provider) podSpecFromService(app, service, release string) (*ac.PodSpec, error) {
	a, err := p.AppGet(app)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if release == "" {
		release = a.Release
	}

	c := ac.Container{
		Env:           []ac.EnvVar{},
		Name:          app,
		Resources:     ac.ResourceRequirements{Requests: ac.ResourceList{}},
		VolumeDevices: []ac.VolumeDevice{},
		VolumeMounts:  []ac.VolumeMount{},
	}

	var vs []ac.Volume

	c.VolumeMounts = append(c.VolumeMounts, ac.VolumeMount{
		Name:      "ca",
		MountPath: "/etc/convox",
	})

	vs = append(vs, ac.Volume{
		Name: "ca",
		VolumeSource: ac.VolumeSource{
			ConfigMap: &ac.ConfigMapVolumeSource{
				LocalObjectReference: ac.LocalObjectReference{
					Name: "ca",
				},
				Optional: options.Bool(true),
			},
		},
	})

	if service != "build" && release != "" {
		m, r, err := common.ReleaseManifest(p, app, release)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		e := structs.Environment{}

		if err := e.Load([]byte(r.Env)); err != nil {
			return nil, errors.WithStack(err)
		}

		env := map[string]string{}

		if s, _ := m.Service(service); s != nil {
			if s.Command != "" {
				parts, err := shellquote.Split(s.Command)
				if err != nil {
					return nil, errors.WithStack(err)
				}
				c.Args = parts
			}

			ee, err := p.environment(a, r, *s, e)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			for k, v := range ee {
				env[k] = v
			}

			repo, _, err := p.Engine.RepositoryHost(app)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			c.Image = fmt.Sprintf("%s:%s.%s", repo, service, r.Build)

			for _, r := range s.ResourceMap() {
				key := r.GetConfigMapKey()
				c.Env = append(c.Env, ac.EnvVar{
					Name: r.Env,
					ValueFrom: &ac.EnvVarSource{
						ConfigMapKeyRef: &ac.ConfigMapKeySelector{
							LocalObjectReference: ac.LocalObjectReference{Name: fmt.Sprintf("resource-%s", nameFilter(r.Name))},
							Key:                  key,
						},
					},
				})
			}

			for _, v := range p.volumeSources(app, s.Name, s.Volumes) {
				vs = append(vs, p.podVolume(app, v))
			}

			for _, v := range s.Volumes {
				to, err := volumeTo(v)
				if err != nil {
					return nil, errors.WithStack(err)
				}

				c.VolumeMounts = append(c.VolumeMounts, ac.VolumeMount{
					Name:      p.volumeName(app, p.volumeFrom(app, s.Name, v)),
					MountPath: to,
				})
			}
		}

		for k, v := range env {
			c.Env = append(c.Env, ac.EnvVar{Name: k, Value: v})
		}
	}

	ps := &ac.PodSpec{
		Containers:            []ac.Container{c},
		ShareProcessNamespace: options.Bool(true),
		Volumes:               vs,
	}

	if service != "build" || !p.BuildDisableResolver {
		if ip, err := p.Engine.ResolverHost(); err == nil {
			ps.DNSPolicy = "None"
			ps.DNSConfig = &ac.PodDNSConfig{
				Nameservers: []string{ip},
				Options: []ac.PodDNSConfigOption{
					{Name: "ndots", Value: options.String("1")},
				},
				Searches: []string{
					fmt.Sprintf("%s.%s.local", app, p.Name),
					fmt.Sprintf("%s.svc.cluster.local", p.AppNamespace(app)),
					fmt.Sprintf("%s.local", p.Name),
					"svc.cluster.local",
					"cluster.local",
				},
			}
		}
	}

	return ps, nil
}

func (p *Provider) podSpecFromRunOptions(app, service string, opts structs.ProcessRunOptions) (*ac.PodSpec, error) {
	s, err := p.podSpecFromService(app, service, common.DefaultString(opts.Release, ""))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if opts.Command != nil {
		parts, err := shellquote.Split(*opts.Command)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		s.Containers[0].Args = parts
	}

	if opts.Environment != nil {
		for k, v := range opts.Environment {
			s.Containers[0].Env = append(s.Containers[0].Env, ac.EnvVar{Name: k, Value: v})
		}
	}

	if opts.Image != nil {
		s.Containers[0].Image = *opts.Image
	}

	if opts.Cpu != nil {
		s.Containers[0].Resources.Requests["cpu"] = resource.MustParse(fmt.Sprintf("%dm", *opts.Cpu))
	}

	if opts.CpuLimit != nil {
		if s.Containers[0].Resources.Limits == nil {
			s.Containers[0].Resources.Limits = ac.ResourceList{}
		}
		s.Containers[0].Resources.Limits["cpu"] = resource.MustParse(fmt.Sprintf("%dm", *opts.CpuLimit))
	}

	if opts.Memory != nil {
		s.Containers[0].Resources.Requests["memory"] = resource.MustParse(fmt.Sprintf("%dMi", *opts.Memory))
	}

	if opts.MemoryLimit != nil {
		if s.Containers[0].Resources.Limits == nil {
			s.Containers[0].Resources.Limits = ac.ResourceList{}
		}
		s.Containers[0].Resources.Limits["memory"] = resource.MustParse(fmt.Sprintf("%dMi", *opts.MemoryLimit))
	}

	if p.DockerUsername != "" && p.DockerPassword != "" {
		_, err = p.CreateOrPatchSecret(p.ctx, am.ObjectMeta{
			Name:      "docker-hub-authentication",
			Namespace: p.AppNamespace(app),
		}, func(s *ac.Secret) *ac.Secret {
			s.Data = map[string][]byte{
				".dockerconfigjson": []byte(fmt.Sprintf(
					`{
				"auths": {
				  "https://index.docker.io/v2/": {
					"auth": "%s" 
				  }
				}
			  }`, base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", p.DockerUsername, p.DockerPassword))))),
			}
			s.Type = "kubernetes.io/dockerconfigjson"
			return s
		}, am.PatchOptions{
			FieldManager: "convox",
		},
		)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		s.ImagePullSecrets = append(s.ImagePullSecrets, ac.LocalObjectReference{Name: "docker-hub-authentication"})
	}

	if opts.Volumes != nil {
		var vs []string

		for from, to := range opts.Volumes {
			vs = append(vs, fmt.Sprintf("%s:%s", from, to))
		}

		for _, v := range p.volumeSources(app, service, vs) {
			s.Volumes = append(s.Volumes, p.podVolume(app, v))
		}

		for _, v := range vs {
			to, err := volumeTo(v)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			s.Containers[0].VolumeMounts = append(s.Containers[0].VolumeMounts, ac.VolumeMount{
				Name:      p.volumeName(app, p.volumeFrom(app, service, v)),
				MountPath: to,
			})
		}
	}

	s.RestartPolicy = "Never"

	return s, nil
}

func (p *Provider) podVolume(app, from string) ac.Volume {
	v := ac.Volume{
		Name: p.volumeName(app, from),
		VolumeSource: ac.VolumeSource{
			PersistentVolumeClaim: &ac.PersistentVolumeClaimVolumeSource{
				ClaimName: p.volumeName(app, from),
			},
		},
	}

	if systemVolume(from) {
		v.VolumeSource = ac.VolumeSource{
			HostPath: &ac.HostPathVolumeSource{
				Path: from,
			},
		}
	}

	return v
}

func (p *Provider) processFromPod(pd ac.Pod) (*structs.Process, error) {
	app := pd.ObjectMeta.Labels["app"]

	c, err := primaryContainer(pd.Spec.Containers, pd.ObjectMeta.Labels["app"])
	if err != nil {
		return nil, err
	}

	status := "unknown"

	switch pd.Status.Phase {
	case "Failed":
		status = "failed"
	case "Pending":
		status = "pending"
	case "Running":
		status = "running"
	case "Succeeded":
		status = "complete"
	}

	if cds := pd.Status.Conditions; len(cds) > 0 && status != "complete" && status != "failed" {
		for _, cd := range cds {
			if cd.Type == "Ready" && cd.Status == "False" {
				status = "unhealthy"
			}
		}
	}

	if css := pd.Status.ContainerStatuses; len(css) > 0 && css[0].Name == app {
		if cs := css[0]; cs.State.Waiting != nil {
			switch cs.State.Waiting.Reason {
			case "CrashLoopBackOff":
				status = "crashed"
			}
		}
	}

	var ports []string
	for _, p := range c.Ports {
		if p.HostPort == 0 {
			ports = append(ports, fmt.Sprint(p.ContainerPort))
		} else {
			ports = append(ports, fmt.Sprintf("%d:%d", p.HostPort, p.ContainerPort))
		}
	}

	ps := &structs.Process{
		Id:       pd.ObjectMeta.Name,
		App:      app,
		Command:  shellquote.Join(c.Args...),
		Host:     pd.Status.PodIP,
		Image:    c.Image,
		Instance: pd.Spec.NodeName,
		Name:     pd.ObjectMeta.Labels["service"],
		Ports:    ports,
		Release:  pd.ObjectMeta.Labels["release"],
		Started:  pd.CreationTimestamp.Time,
		Status:   status,
	}

	if ps.App == "system" {
		ps.Release = p.Version
	}

	return ps, nil
}

type terminalSize struct {
	Height int
	Width  int
	sent   bool
}

func (ts *terminalSize) Next() *remotecommand.TerminalSize {
	if ts.sent {
		return nil
	}

	ts.sent = true

	return &remotecommand.TerminalSize{Height: uint16(ts.Height), Width: uint16(ts.Width)}
}
