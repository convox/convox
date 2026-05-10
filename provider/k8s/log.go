package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/scheme"
)

func (p *Provider) systemLog(tid, app, name string, ts time.Time, message string) error {
	if tid != "" {
		app = fmt.Sprintf("%s-%s", tid, app)
	}
	if options.GetFeatureGates()[options.FeatureGateTid] && tid == "" {
		return p.Engine.Log(app, fmt.Sprintf("system/%s", name), ts, message)
	}
	return p.Engine.Log(app, fmt.Sprintf("system/k8s/%s", name), ts, message)
}

func (p *Provider) ServiceLogs(app, name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	selector := fmt.Sprintf("app=%s,name=%s,type=service", app, name)
	return p.streamPodLogs(app, selector, opts, "ServiceLogs",
		fmt.Sprintf("app: %s service: %s", app, name))
}

// AppLogs streams logs from every pod backing the app's services. The
// selector matches pods labeled `app=<app>,type=service`, omitting the
// `name=<svc>` filter so all services are interleaved.
//
// Earlier rack versions returned ErrNotImplemented here, so a user
// running `convox logs -a my-app` saw no output and no error. The
// selector-based fan-out fixes the empty-output regression — the
// underlying kubectl/log infrastructure (newlogsConfigFlags + RunLogs)
// is the same path ServiceLogs already uses, so the behaviour matches
// `convox logs -a my-app -s <each>` interleaved.
func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	selector := fmt.Sprintf("app=%s,type=service", name)
	return p.streamPodLogs(name, selector, opts, "AppLogs", fmt.Sprintf("app: %s", name))
}

// streamPodLogs is the shared kubectl-logs entry point used by both
// AppLogs and ServiceLogs. The caller passes a label selector and a
// log-context label so error/completion lines distinguish the two
// surfaces in rack logs.
func (p *Provider) streamPodLogs(app, selector string, opts structs.LogsOptions, logCtx, ctxDetail string) (io.ReadCloser, error) {
	r, w := io.Pipe()
	logOpts := logs.NewLogsOptions(genericiooptions.IOStreams{
		In:     r,
		Out:    w,
		ErrOut: w,
	})

	f := cmdutil.NewFactory(p.newlogsConfigFlags(p.AppNamespace(app)))

	if err := p.configureLogOptionsBySelector(app, selector, f, logOpts, opts); err != nil {
		return nil, err
	}

	go func() {
		if err := logOpts.RunLogs(); err != nil {
			w.CloseWithError(err)
			_ = p.logger.At(logCtx).Errorf("%s err: %s", ctxDetail, err)
		} else {
			p.logger.At(logCtx).Logf("complete")
			w.CloseWithError(nil)
		}
	}()

	return r, nil
}

func (p *Provider) configureLogOptionsBySelector(app, selector string, f cmdutil.Factory, o *logs.LogsOptions, logConfig structs.LogsOptions) error {
	var err error
	o.Follow = true
	if logConfig.Follow != nil && !*logConfig.Follow {
		o.Follow = false
	}

	o.Prefix = true
	o.Timestamps = true
	if logConfig.Prefix != nil && !*logConfig.Prefix {
		o.Prefix = false
		o.Timestamps = false
	}

	if logConfig.Since != nil {
		o.SinceSeconds = *logConfig.Since
	}

	if logConfig.Tail != nil {
		o.Tail = int64(*logConfig.Tail)
	}

	if logConfig.Previous != nil && *logConfig.Previous {
		o.Previous = true
	}

	o.MaxFollowConcurrency = 20
	if logConfig.MaxLogRequests != nil {
		o.MaxFollowConcurrency = *logConfig.MaxLogRequests
	}

	o.Namespace = p.AppNamespace(app)
	o.ConsumeRequestFn = logs.DefaultConsumeRequest
	o.GetPodTimeout = 20 * time.Second
	o.Selector = selector
	o.Options, err = o.ToLogOptions()
	if err != nil {
		return err
	}

	logOptsData, _ := json.Marshal(o.Options)
	p.logger.At("configureLogOptionsBySelector").Logf("selector=%q log options %s", selector, string(logOptsData))

	o.RESTClientGetter = f
	o.LogsForObject = polymorphichelpers.LogsForObjectFn

	if o.Object == nil {
		builder := f.NewBuilder().
			WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
			NamespaceParam(o.Namespace).DefaultNamespace().
			SingleResourceType()
		if o.Selector != "" {
			builder.ResourceTypes("pods").LabelSelectorParam(o.Selector)
		}
		infos, err := builder.Do().Infos()
		if err != nil {
			return err
		}
		if o.Selector == "" && len(infos) != 1 {
			return structs.ErrBadRequest("expected a resource")
		}
		o.Object = infos[0].Object
		if o.Selector != "" && len(o.Object.(*corev1.PodList).Items) == 0 {
			return structs.ErrNotFound("no resources found in %s namespace", o.Namespace)
		}
	}

	return nil
}

func (p *Provider) newlogsConfigFlags(ns string) *genericclioptions.ConfigFlags {
	cf := genericclioptions.NewConfigFlags(true)
	cf.Insecure = options.Bool(true)
	cf.KubeConfig = nil
	cf.APIServer = options.String(p.Config.Host)
	// Read token fresh from disk to handle EKS projected service account
	// token rotation. The BearerTokenFile is updated by the kubelet when
	// the token is rotated, but BearerToken is a static string from startup.
	if p.Config.BearerTokenFile != "" {
		if token, err := os.ReadFile(p.Config.BearerTokenFile); err == nil {
			cf.BearerToken = options.String(strings.TrimSpace(string(token)))
		} else {
			cf.BearerToken = options.String(p.Config.BearerToken)
		}
	} else {
		cf.BearerToken = options.String(p.Config.BearerToken)
	}
	cf.CAFile = options.String(p.Config.CAFile)
	cf.CertFile = options.String(p.Config.CertFile)
	cf.DisableCompression = options.Bool(p.Config.DisableCompression)
	cf.Impersonate = options.String(p.Config.Impersonate.UserName)
	cf.ImpersonateGroup = &p.Config.Impersonate.Groups
	cf.ImpersonateUID = options.String(p.Config.Impersonate.UID)
	cf.Insecure = &p.Config.Insecure
	cf.KeyFile = &p.Config.KeyFile
	cf.Namespace = &ns
	cf.Password = &p.Config.Password
	cf.TLSServerName = &p.Config.TLSClientConfig.ServerName
	cf.Timeout = options.String(p.Config.Timeout.String())
	cf.Username = &p.Config.Username
	return cf
}
