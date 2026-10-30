package k8s_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
)

func TestObjectStorageContained(t *testing.T) {
	p := &k8s.Provider{Storage: t.TempDir()}

	_, err := p.ObjectStore("app1", "build/B1/logs", strings.NewReader("logs"), structs.ObjectStoreOptions{})
	require.NoError(t, err)

	r, err := p.ObjectFetch("app1", "/build/B1/logs")
	require.NoError(t, err)
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	require.Equal(t, "logs", string(data))

	require.NoError(t, os.WriteFile(filepath.Join(p.Storage, "secret"), []byte("secret"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(p.Storage, "objectsX"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(p.Storage, "objectsX", "f"), []byte("secret"), 0600))

	tests := []struct {
		App string
		Key string
	}{
		{App: "..", Key: "/secret"},
		{App: "app1", Key: "/../../secret"},
		{App: "", Key: "/../objectsX/f"},
		{App: "", Key: ""},
	}

	for _, test := range tests {
		_, err := p.ObjectFetch(test.App, test.Key)
		require.EqualError(t, err, "invalid object key: "+test.Key, "app=%q key=%q", test.App, test.Key)
	}
}
