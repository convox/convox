package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvironmentStringMasked(t *testing.T) {
	env := Environment{
		"FOO":       "bar",
		"API_TOKEN": "sk-live-abc123",
		"DB_URL":    "postgres://user:pass@host/db",
		"PORT":      "3000",
	}

	masked := map[string]bool{
		"API_TOKEN": true,
		"DB_URL":    true,
	}

	result := env.StringMasked(masked)
	require.Equal(t, "API_TOKEN=****\nDB_URL=****\nFOO=bar\nPORT=3000", result)
}

func TestEnvironmentStringMaskedEmpty(t *testing.T) {
	env := Environment{
		"FOO": "bar",
		"BAZ": "quux",
	}

	result := env.StringMasked(map[string]bool{})
	require.Equal(t, env.String(), result)
}

func TestEnvironmentStringMaskedNilMap(t *testing.T) {
	env := Environment{
		"FOO": "bar",
		"BAZ": "quux",
	}

	result := env.StringMasked(nil)
	require.Equal(t, env.String(), result)
}

func TestEnvironmentStringMaskedKeyNotInEnv(t *testing.T) {
	env := Environment{
		"FOO": "bar",
	}

	masked := map[string]bool{
		"NONEXISTENT": true,
	}

	result := env.StringMasked(masked)
	require.Equal(t, "FOO=bar", result)
}

func TestEnvironmentStringMaskedAllKeys(t *testing.T) {
	env := Environment{
		"SECRET1": "val1",
		"SECRET2": "val2",
	}

	masked := map[string]bool{
		"SECRET1": true,
		"SECRET2": true,
	}

	result := env.StringMasked(masked)
	require.Equal(t, "SECRET1=****\nSECRET2=****", result)
}

func TestEnvironmentStringUnchanged(t *testing.T) {
	env := Environment{
		"FOO": "bar",
		"BAZ": "quux",
	}

	require.Equal(t, "BAZ=quux\nFOO=bar", env.String())
}

func TestEnvironmentStringMaskedDoesNotMutate(t *testing.T) {
	env := Environment{
		"API_TOKEN": "sk-live-abc123",
		"FOO":       "bar",
	}

	masked := map[string]bool{"API_TOKEN": true}

	result := env.StringMasked(masked)
	require.Equal(t, "API_TOKEN=****\nFOO=bar", result)

	require.Equal(t, "sk-live-abc123", env["API_TOKEN"])

	require.Equal(t, "API_TOKEN=sk-live-abc123\nFOO=bar", env.String())
}
