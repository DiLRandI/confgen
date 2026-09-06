package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCLI(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.yaml")
	out := filepath.Join(dir, "config_gen.go")
	require.NoError(t, os.WriteFile(schema, []byte("version: 1\npackage: app\nfields: {port: {type: int, default: 8080}}"), 0o600))
	var stderr bytes.Buffer
	{
		code := run([]string{"-schema", schema, "-out", out, "-example-yaml", filepath.Join(dir, "example.yaml"), "-example-env", filepath.Join(dir, "env.example")}, &stderr)
		require.Equal(t, 0, code, "%d %s", code, stderr.String())
	}
	before, _ := os.ReadFile(out)
	for _, args := range [][]string{{}, {"-schema", schema, "-out", schema}, {"-schema", schema, "-out", out, "-example-env", out}, {"-unknown"}} {
		stderr.Reset()
		{
			code := run(args, &stderr)
			require.NotEqual(t, 0, code, "accepted %v", args)
			require.NotEqual(t, 0, stderr.Len(), "accepted %v", args)
		}
	}
	require.NoError(t, os.WriteFile(schema, []byte("version: 1\npackage: app\nfields: {x: {type: duration, default: wrong}}"), 0o600))
	stderr.Reset()
	require.NotEqual(t, 0, run([]string{"-schema", schema, "-out", out}, &stderr), "bad error behavior")
	require.NotContains(t, stderr.String(), "panic", "bad error behavior")
	after, _ := os.ReadFile(out)
	require.Equal(t, before, after, "failed schema replaced output")
}

func TestHelp(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"generate", "-h"}} {
		var b bytes.Buffer
		require.Equal(t, 0, run(args, &b), b.String())
		require.Contains(t, b.String(), "confgen generate", b.String())
		require.Contains(t, b.String(), "-schema", b.String())
	}
}

func TestGenerateFromDoesNotCopyDefaultsUnlessRequested(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "config.yaml")
	original := []byte("server: {port: 8080, host: localhost}\norigins: [a, b]\npassword: sentinel-secret\n")
	require.NoError(t, os.WriteFile(input, original, 0o600))
	without := filepath.Join(dir, "without.go")
	with := filepath.Join(dir, "with.go")
	var stderr bytes.Buffer
	require.Equal(t, 0, run([]string{"generate", "--from", input, "--out", without}, &stderr), stderr.String())
	require.Equal(t, 0, run([]string{"generate", "--from", input, "--copy-defaults", "--out", with}, &stderr), stderr.String())
	withoutCode, err := os.ReadFile(without)
	require.NoError(t, err)
	withCode, err := os.ReadFile(with)
	require.NoError(t, err)
	require.NotContains(t, string(withoutCode), "HasDefault: true", "generate --from default policy is incorrect")
	require.Contains(t, string(withCode), "HasDefault: true", "generate --from default policy is incorrect")
	require.NotContains(t, string(withoutCode), "sentinel-secret", "generate --from copied an input secret without opt-in")
	require.Contains(t, string(withCode), "sentinel-secret", "generate --from copied an input secret without opt-in")
	unchanged, err := os.ReadFile(input)
	require.NoError(t, err)
	require.Equal(t, original, unchanged, "generate --from changed input")
}
