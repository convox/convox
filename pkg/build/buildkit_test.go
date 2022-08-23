package build_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/convox/convox/pkg/build"
	"github.com/convox/convox/sdk"
	"github.com/stretchr/testify/require"
)

func TestLogin(t *testing.T) {
	tmp, err := os.MkdirTemp("", "convox-tests")
	if err != nil {
		panic(err)
	}
	err = os.Mkdir(fmt.Sprintf("%s/.docker", tmp), 0777)
	if err != nil {
		panic(err)
	}

	defer os.Remove(tmp)
	os.Setenv("HOME", tmp)

	r, err := sdk.NewFromEnv()
	if err != nil {
		panic(err)
	}

	bk := build.BuildKit{}
	opts := build.Options{
		App:      "app1",
		Auth:     `{"host1":{"username":"user1","password":"pass1"}}`,
		Cache:    true,
		Id:       "build1",
		Manifest: "convox2.yml",
		Push:     "push1",
		Rack:     "rack1",
		Source:   "object://app1/object.tgz",
	}
	bb, err := build.New(r, opts, &bk)
	if err != nil {
		panic(err)
	}

	err = bk.Login(bb)
	if err != nil {
		panic(err)
	}

	f, err := os.Open(fmt.Sprintf("%s/.docker/config.json", tmp))
	if err != nil {
		panic(err)
	}

	data, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}

	// base64 encoded user:password
	require.Equal(t, string(data), `{"Auths":{"host1":{"auth":"dXNlcjE6cGFzczE="}}}`)
}
