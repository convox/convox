package sdk_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/stretchr/testify/require"
)

func TestInstanceKeyroll(t *testing.T) {
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(&structs.KeyPair{
			Name:       options.String("test"),
			PrivateKey: options.String("test"),
		})
		return
	}))
	defer ht.Close()

	c, err := sdk.New(ht.URL)
	require.NoError(t, err)

	v, err := c.InstanceKeyroll()
	require.NoError(t, err)
	require.NotNil(t, v)
	require.Equal(t, "test", *v.Name)
	require.Equal(t, "test", *v.PrivateKey)
}

func TestInstanceKeyrollEmptyBody(t *testing.T) {
	ht := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		return
	}))
	defer ht.Close()

	c, err := sdk.New(ht.URL)
	require.NoError(t, err)

	v, err := c.InstanceKeyroll()
	require.NoError(t, err)
	require.NotNil(t, v)
}
