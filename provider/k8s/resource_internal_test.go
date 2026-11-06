package k8s

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func exportOutput(t *testing.T, command string, args ...string) string {
	t.Helper()

	rr, ww := io.Pipe()
	go resourceExportCommand(ww, command, args...)

	data, err := io.ReadAll(rr)
	require.NoError(t, err)
	return string(data)
}

func TestResourceExportCommandKeepsStderrOutOfTheDump(t *testing.T) {
	require.Equal(t, "out\n", exportOutput(t, "sh", "-c", "echo out; echo warn >&2"))
}

func TestResourceExportCommandReportsStderrOnFailure(t *testing.T) {
	require.Equal(t, "out\nERROR: could not export: exit status 3: warn\n", exportOutput(t, "sh", "-c", "echo out; echo warn >&2; exit 3"))
	require.Equal(t, "ERROR: could not export: exit status 3\n", exportOutput(t, "sh", "-c", "exit 3"))
}

func TestMysqldumpArgs(t *testing.T) {
	cn := &resourceConnection{Database: "app", Host: "db", Password: "pw", Port: "3306", Username: "root"}
	base := []string{"-h", "db", "-P", "3306", "-u", "root", "-ppw"}

	for _, typ := range []string{"mysql", "rds-mysql"} {
		require.Equal(t, append(append([]string{}, base...), "app"), mysqldumpArgs(typ, cn), typ)
	}

	for _, typ := range []string{"mariadb", "rds-mariadb"} {
		require.Equal(t, append(append([]string{}, base...), "--column-statistics=0", "app"), mysqldumpArgs(typ, cn), typ)
	}
}
