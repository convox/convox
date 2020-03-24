package k8s_test

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cv "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestReleasePromote(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kc := p.Convox.(*cvfake.Clientset)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		require.NoError(t, buildCreate(kc, "rack1-app1", "build1", "basic"))
		require.NoError(t, releaseCreate(kc, "rack1-app1", "release1", "basic"))
		require.NoError(t, releaseCreate(kc, "rack1-app1", "release2", "basic"))

		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)
		require.NoError(t, releaseApply(aa, "rack1-app1", "release2", "app", "basic-app"))

		err := p.ReleasePromote("app1", "release2", structs.ReleasePromoteOptions{})
		require.NoError(t, err)
	})
}

func releaseApply(aa *atom.MockInterface, ns, id, atom, fixture string) error {
	data, err := ioutil.ReadFile(fmt.Sprintf("testdata/release-%s.yml", fixture))
	if err != nil {
		return errors.WithStack(err)
	}

	aa.On("Apply", ns, atom, id, data, int32(1800)).Return(nil).Once()

	return nil
}

func releaseCreate(kc cv.Interface, ns, id, fixture string) error {
	spec, err := releaseFixture(fixture)
	if err != nil {
		return errors.WithStack(err)
	}

	r := &ca.Release{
		ObjectMeta: am.ObjectMeta{
			Name: id,
		},
		Spec: *spec,
	}

	if _, err := kc.ConvoxV1().Releases(ns).Create(r); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func releaseFixture(name string) (*ca.ReleaseSpec, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("testdata/release-%s.yml", name))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var fixture struct {
		Build    string
		Created  string
		Env      map[string]string
		Manifest interface{}
	}

	if err := yaml.Unmarshal(data, &fixture); err != nil {
		return nil, errors.WithStack(err)
	}

	ep := []string{}

	for k, v := range fixture.Env {
		ep = append(ep, fmt.Sprintf("%s=%s", k, v))
	}

	sort.Strings(ep)

	mdata, err := yaml.Marshal(fixture.Manifest)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := &ca.ReleaseSpec{
		Build:    fixture.Build,
		Created:  fixture.Created,
		Env:      strings.Join(ep, "\n"),
		Manifest: string(mdata),
	}

	return s, nil
}
