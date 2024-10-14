package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	atomv1 "github.com/convox/convox/pkg/atom/pkg/apis/atom/v1"
	av "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/convox/logger"
)

type AtomController struct {
	provider *Provider

	atom                av.Interface
	logger              *logger.Logger
	start               time.Time
	dependencyProcessor *sync.Map
}

func NewAtomController(p *Provider) (*AtomController, error) {
	atom, err := av.NewForConfig(p.Config)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ac := &AtomController{
		atom:                atom,
		dependencyProcessor: &sync.Map{},
		provider:            p,
		logger:              logger.New("ns=atom-controller"),
		start:               time.Now().UTC(),
	}

	return ac, nil
}

func (a *AtomController) Run() error {
	return RunUsingLeaderElection(a.provider.Namespace, "convox-atom-controller", a.provider.Cluster, a.Start, a.Stop)
}

func (a *AtomController) Start(ctx context.Context) {
	for {
		a.sync()
		time.Sleep(30 * time.Second)
	}
}

func (a *AtomController) Stop() {
}

func (a *AtomController) sync() error {
	a.logger.Logf("syncing pending atoms...")
	listResp, err := a.atom.AtomV1().Atoms(v1.NamespaceAll).List(a.provider.ctx, v1.ListOptions{})
	if err != nil {
		a.logger.Logf("failed to synce atom: %s", err)
		return err
	}

	for i := range listResp.Items {
		obj := &listResp.Items[i]
		// obj.Name is fixed, so obj.Namespace will be unique per app
		if _, ok := a.dependencyProcessor.Load(obj.Namespace); !ok && len(obj.Spec.Dependencies) > 0 {
			a.dependencyProcessor.Store(obj.Namespace, true)
			go a.processDependency(&listResp.Items[i])
		}
	}
	return nil
}

func (a *AtomController) processDependency(obj *atomv1.Atom) {
	a.logger.Logf("start processing dependency for: %s", obj.Namespace)
	defer a.dependencyProcessor.Delete(obj.Namespace)
	for _, dep := range obj.Spec.Dependencies {
		app, rType, _ := parseResourceSubstitutionId(dep)

		if strings.HasPrefix(rType, "rds-") {
			if err := a.processRdsDependency(obj, dep); err != nil {
				a.logger.Logf(err.Error())
				a.provider.systemLog(app, "state", time.Now(), err.Error())
				return
			}
		}

		if strings.HasPrefix(rType, "elasticache-") {
			if err := a.processElasticacheDependency(obj, dep); err != nil {
				a.logger.Logf(err.Error())
				a.provider.systemLog(app, "state", time.Now(), err.Error())
				return
			}
		}
	}
	_, err := a.PatchAtom(a.provider.ctx, obj, func(atm *atomv1.Atom) *atomv1.Atom {
		atm.Spec.Dependencies = nil
		return atm
	}, v1.PatchOptions{})
	if err != nil {
		a.logger.Logf(err.Error())
		a.provider.systemLog(strings.TrimPrefix(obj.Namespace, a.provider.Name), "state", time.Now(), fmt.Sprintf("failed to patch atom dependency: %s", err))
		return
	}
}

func (a *AtomController) processRdsDependency(obj *atomv1.Atom, dep string) error {
	app, rType, rName := parseResourceSubstitutionId(dep)
	resourceId := a.provider.CreateAwsResourceStateId(app, rName)

	conn, err := a.provider.RdsProvisioner.GetConnectionInfo(resourceId)
	if err != nil {
		return fmt.Errorf("failed to get connection info for resource '%s': %s", rName, err)
	}
	data, err := a.provider.releaseTemplateCustomResource(&structs.App{
		Name: app,
	}, nil, manifest.Resource{
		Type: rType,
		Name: rName,
	}, conn)
	if err != nil {
		return fmt.Errorf("failed to create resource '%s' connecton config: %s", rName, err)
	}
	if err := a.resolveDependencyInAtomVersion(obj, dep, data); err != nil {
		return err
	}
	return nil
}

func (a *AtomController) processElasticacheDependency(obj *atomv1.Atom, dep string) error {
	app, rType, rName := parseResourceSubstitutionId(dep)
	resourceId := a.provider.CreateAwsResourceStateId(app, rName)

	conn, err := a.provider.ElasticacheProvisioner.GetConnectionInfo(resourceId)
	if err != nil {
		return fmt.Errorf("failed to get connection info for resource '%s': %s", rName, err)
	}
	data, err := a.provider.releaseTemplateCustomResource(&structs.App{
		Name: app,
	}, nil, manifest.Resource{
		Type: rType,
		Name: rName,
	}, conn)
	if err != nil {
		return fmt.Errorf("failed to create resource '%s' connecton config: %s", rName, err)
	}
	if err := a.resolveDependencyInAtomVersion(obj, dep, data); err != nil {
		return err
	}
	return nil
}

func (a *AtomController) resolveDependencyInAtomVersion(obj *atomv1.Atom, dep string, data []byte) error {
	atomVersion, err := a.atom.AtomV1().AtomVersions(obj.Namespace).Get(a.provider.ctx, obj.Spec.CurrentVersion, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get atom version '%s': %s", obj.Spec.CurrentVersion, err)
	}

	atomVersion.Spec.Template = bytes.Replace(atomVersion.Spec.Template, []byte(dep), data, -1)

	_, err = a.atom.AtomV1().AtomVersions(atomVersion.Namespace).Update(a.provider.ctx, atomVersion, v1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch atom version template '%s': %s", obj.Spec.CurrentVersion, err)
	}
	return err
}

func (a *AtomController) PatchAtom(ctx context.Context, cur *atomv1.Atom, transform func(*atomv1.Atom) *atomv1.Atom, opts metav1.PatchOptions) (*atomv1.Atom, error) {
	return a.PatchAtomObject(ctx, cur, transform(cur.DeepCopy()), opts)
}

func (a *AtomController) PatchAtomObject(ctx context.Context, cur, mod *atomv1.Atom, opts metav1.PatchOptions) (*atomv1.Atom, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, atomv1.Atom{})
	if err != nil {
		return nil, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, nil
	}
	a.logger.Logf("Patching Atom %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	return a.atom.AtomV1().Atoms(cur.Namespace).Patch(ctx, cur.Name, types.MergePatchType, patch, opts)
}
