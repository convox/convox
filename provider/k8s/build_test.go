package k8s_test

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cv "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildList(t *testing.T) {
	tests := []struct {
		Name        string
		RackName    string
		AppName     string
		AppNameList string
		Namespace   string
		BuildName   string
		Response    structs.Builds
		Err         error
	}{
		{
			Name:        "Success",
			RackName:    "rack1",
			AppName:     "app1",
			AppNameList: "app1",
			Namespace:   "rack1-app1",
			BuildName:   "build1",
			Response:    structs.Builds{structs.Build{Id: "BUILD1", App: "app1", Description: "foo", Entrypoint: "", Logs: "", Manifest: "services:\n  web:\n    build: .\n    port: 5000\n", Process: "", Release: "", Reason: "", Repository: "", Status: "", Started: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Ended: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Tags: map[string]string(nil)}},
			Err:         nil,
		},
		{
			Name:        "app not found",
			RackName:    "rack2",
			AppName:     "app2",
			AppNameList: "app2-not-found",
			Namespace:   "rack2-app2",
			BuildName:   "build2",
			Response:    structs.Builds(structs.Builds{}),
			Err:         errors.New("app not found: app2-not-found"),
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				if test.Err == nil {
					aa := p.Atom.(*atom.MockInterface)
					aa.On("Status", test.Namespace, "app").Return("Updating", "R1234567", nil).Once()
				}

				err := buildCreate(p.Convox, test.Namespace, test.BuildName, "basic")
				require.NoError(t, err)

				bs, err := p.BuildList(test.AppNameList, structs.BuildListOptions{})

				if err == nil {
					require.NoError(t, err)
					assert.Equal(t, bs, test.Response)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestBuildGet(t *testing.T) {
	tests := []struct {
		Name        string
		RackName    string
		AppName     string
		AppNameList string
		Namespace   string
		BuildName   string
		Response    *structs.Build
		Err         error
	}{
		{
			Name:        "Success",
			RackName:    "rack1",
			AppName:     "app1",
			AppNameList: "app1",
			Namespace:   "rack1-app1",
			BuildName:   "build1",
			Response:    &structs.Build{Id: "BUILD1", App: "app1", Description: "foo", Entrypoint: "", Logs: "", Manifest: "services:\n  web:\n    build: .\n    port: 5000\n", Process: "", Release: "", Reason: "", Repository: "", Status: "", Started: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Ended: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Tags: map[string]string(nil)},
			Err:         nil,
		},
		{
			Name:        "app not found",
			RackName:    "rack2",
			AppName:     "app2",
			AppNameList: "app2-not-found",
			Namespace:   "rack2-app2",
			BuildName:   "build2",
			Response:    nil,
			Err:         errors.New("builds.convox.com \"build2\" not found"),
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				err := buildCreate(p.Convox, test.Namespace, test.BuildName, "basic")
				require.NoError(t, err)

				bs, err := p.BuildGet(test.AppName, test.BuildName)
				if err == nil {
					require.NoError(t, err)
					assert.Equal(t, bs, test.Response)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestBuildUpdate(t *testing.T) {
	tests := []struct {
		Name        string
		RackName    string
		AppName     string
		AppNameList string
		Namespace   string
		BuildName   string
		Response    *structs.Build
		Err         error
	}{
		{
			Name:        "Success",
			RackName:    "rack1",
			AppName:     "app1",
			AppNameList: "app1",
			Namespace:   "rack1-app1",
			BuildName:   "build1",
			Response:    &structs.Build{Id: "BUILD1", App: "", Description: "foo", Entrypoint: "", Logs: "", Manifest: "services:\n  web:\n    build: .\n    port: 5000\n", Process: "", Release: "", Reason: "", Repository: "", Status: "", Started: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Ended: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Tags: map[string]string(nil)},
			Err:         nil,
		},
		{
			Name:        "app not found",
			RackName:    "rack2",
			AppName:     "app2",
			AppNameList: "app2-not-found",
			Namespace:   "rack2-app2",
			BuildName:   "build2",
			Response:    nil,
			Err:         errors.New("builds.convox.com \"build2\" not found"),
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				err := buildCreate(p.Convox, test.Namespace, test.BuildName, "basic")
				require.NoError(t, err)

				status := "Running"
				release := "v1"

				_, err = p.BuildUpdate(test.AppName, test.BuildName, structs.BuildUpdateOptions{
					Status:  &status,
					Release: &release,
				})

				if err == nil {
					require.NoError(t, err)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestBuildCreate(t *testing.T) {
	tests := []struct {
		Name        string
		RackName    string
		AppName     string
		AppNameList string
		Namespace   string
		BuildName   string
		Response    *structs.Build
		Err         error
	}{
		{
			Name:        "Success",
			RackName:    "rack1",
			AppName:     "app1",
			AppNameList: "app1",
			Namespace:   "rack1-app1",
			BuildName:   "build1",
			Response:    &structs.Build{Id: "BUILD1", App: "", Description: "foo", Entrypoint: "", Logs: "", Manifest: "services:\n  web:\n    build: .\n    port: 5000\n", Process: "", Release: "", Reason: "", Repository: "", Status: "", Started: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Ended: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Tags: map[string]string(nil)},
			Err:         nil,
		},
		{
			Name:        "app not found",
			RackName:    "rack2",
			AppName:     "app2",
			AppNameList: "app2-not-found",
			Namespace:   "rack2-app2",
			BuildName:   "build2",
			Response:    nil,
			Err:         errors.New("app not found: app2"),
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				if test.Err == nil {
					aa := p.Atom.(*atom.MockInterface)
					aa.On("Status", test.Namespace, "app").Return("Creating", "R1234567", nil).Times(3)
				}

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				_, err := p.BuildCreate(test.AppName, test.BuildName, structs.BuildCreateOptions{})
				if err == nil {
					require.NoError(t, err)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func buildCreate(kc cv.Interface, ns, id, fixture string) error {
	spec, err := buildFixture(fixture)
	if err != nil {
		return errors.WithStack(err)
	}

	app := strings.Split(ns, "-")

	b := &ca.Build{
		ObjectMeta: am.ObjectMeta{
			Name: id,
			Labels: map[string]string{
				"app": app[1],
			},
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
		Description string
		Ended       string
		Manifest    interface{}
		Started     string
	}

	if err := yaml.Unmarshal(data, &fixture); err != nil {
		return nil, errors.WithStack(err)
	}

	mdata, err := yaml.Marshal(fixture.Manifest)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := &ca.BuildSpec{
		Description: fixture.Description,
		Ended:       fixture.Ended,
		Manifest:    string(mdata),
		Started:     fixture.Started,
	}

	return s, nil
}
