package k8s

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
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
	return p.Engine.Log(app, fmt.Sprintf("system/k8s/%s", name), ts, message)
}

func (p *Provider) ServiceLogs(app, name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	r, w := io.Pipe()
	logOpts := logs.NewLogsOptions(genericiooptions.IOStreams{
		In:     r,
		Out:    w,
		ErrOut: w,
	})

	f := cmdutil.NewFactory(p.newlogsConfigFlags(p.AppNamespace(app)))

	if err := p.configureLogOptionsForService(app, name, f, logOpts, opts); err != nil {
		return nil, err
	}

	go func() {
		if err := logOpts.RunLogs(); err != nil {
			w.CloseWithError(err)
			p.logger.At("ServiceLogs").Errorf("app: %s service: %s err: %s", app, name, err)
		} else {
			p.logger.At("ServiceLogs").Logf("complete")
			w.CloseWithError(nil)
		}
	}()

	return r, nil
}

func (p *Provider) configureLogOptionsForService(app, name string, f cmdutil.Factory, o *logs.LogsOptions, logConfig structs.LogsOptions) error {
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

	o.Namespace = p.AppNamespace(app)
	o.ConsumeRequestFn = logs.DefaultConsumeRequest
	o.GetPodTimeout = 20 * time.Second
	o.Selector = fmt.Sprintf("app=%s,name=%s,type=service", app, name)
	o.Options, err = o.ToLogOptions()
	if err != nil {
		return err
	}

	logOptsData, _ := json.Marshal(o.Options)
	p.logger.At("configureLogOptionsForService").Logf("log options %s", string(logOptsData))

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
			return errors.New("expected a resource")
		}
		o.Object = infos[0].Object
		if o.Selector != "" && len(o.Object.(*corev1.PodList).Items) == 0 {
			return fmt.Errorf("no resources found in %s namespace", o.Namespace)
		}
	}

	return nil
}

func (p *Provider) newlogsConfigFlags(ns string) *genericclioptions.ConfigFlags {
	cf := genericclioptions.NewConfigFlags(true)
	cf.Insecure = options.Bool(true)
	cf.KubeConfig = nil
	cf.APIServer = options.String(p.Config.Host)
	cf.BearerToken = options.String(p.Config.BearerToken)
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
