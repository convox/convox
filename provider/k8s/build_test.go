package k8s_test

import (
	"fmt"
	"io/ioutil"

	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cv "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildCreate(kc cv.Interface, ns, id, fixture string) error {
	spec, err := buildFixture(fixture)
	if err != nil {
		return errors.WithStack(err)
	}

	b := &ca.Build{
		ObjectMeta: am.ObjectMeta{
			Name: id,
		},
		Spec: *spec,
	}

	if _, err := kc.ConvoxV1().Builds(ns).Create(b); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func buildFixture(name string) (*ca.BuildSpec, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("testdata/build-%s.yml", name))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var fixture struct {
		Ended    string
		Manifest interface{}
		Started  string
	}

	if err := yaml.Unmarshal(data, &fixture); err != nil {
		return nil, errors.WithStack(err)
	}

	mdata, err := yaml.Marshal(fixture.Manifest)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := &ca.BuildSpec{
		Ended:    fixture.Ended,
		Manifest: string(mdata),
		Started:  fixture.Started,
	}

	return s, nil
}
