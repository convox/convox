package k8s

import (
	"fmt"
	"io"

	"github.com/convox/convox/pkg/structs"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) SystemGet() (*structs.System, error) {
	status := "running"

	// status, err := p.Engine.SystemStatus()
	// if err != nil {
	// 	return nil, err
	// }

	// ss, _, err := p.atom.Status(p.Namespace, "system")
	// if err != nil {
	// 	return nil, err
	// }

	// status = "running"

	// switch status {
	// case "running", "unknown":
	// 	status = common.AtomStatus(ss)
	// }

	s := &structs.System{
		Domain:   fmt.Sprintf("router.%s", p.Domain),
		Name:     p.Name,
		Provider: p.Provider,
		Status:   status,
		Version:  p.Version,
	}

	return s, nil
}

func (p *Provider) SystemInstall(w io.Writer, opts structs.SystemInstallOptions) (string, error) {
	return "", fmt.Errorf("unimplemented")
}

func (p *Provider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *Provider) SystemMetrics(opts structs.MetricsOptions) (structs.Metrics, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *Provider) SystemProcesses(opts structs.SystemProcessesOptions) (structs.Processes, error) {
	pds, err := p.Cluster.CoreV1().Pods(p.Namespace).List(am.ListOptions{})
	if err != nil {
		return nil, err
	}

	pss := structs.Processes{}

	for _, pd := range pds.Items {
		ps, err := processFromPod(pd)
		if err != nil {
			return nil, err
		}

		ps.App = "rack"
		ps.Release = p.Version

		pss = append(pss, *ps)
	}

	pds, err = p.Cluster.CoreV1().Pods("convox-system").List(am.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pd := range pds.Items {
		ps, err := processFromPod(pd)
		if err != nil {
			return nil, err
		}

		ps.App = "system"
		ps.Release = p.Version

		pss = append(pss, *ps)
	}

	return pss, nil
}

func (p *Provider) SystemReleases() (structs.Releases, error) {
	return nil, fmt.Errorf("release history is unavailable")
}

func (p *Provider) SystemUninstall(name string, w io.Writer, opts structs.SystemUninstallOptions) error {
	return fmt.Errorf("unimplemented")
}

func (p *Provider) SystemUpdate(opts structs.SystemUpdateOptions) error {
	return fmt.Errorf("console update not yet supported")
}
