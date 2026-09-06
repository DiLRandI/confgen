package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/DiLRandI/confgen/input"
	"github.com/stretchr/testify/require"
)

func TestCLIInputLimits(t *testing.T) {
	dir := t.TempDir()
	large := filepath.Join(dir, "large.yaml")
	f, err := os.Create(large)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(int64(input.DefaultLimit)+1))
	require.NoError(t, f.Close())
	small := filepath.Join(dir, "small.yaml")
	require.NoError(t, os.WriteFile(small, []byte("value: hello\n"), 0600))
	for _, args := range [][]string{
		{"generate", "--schema", large},
		{"generate", "--from", large},
		{"init", "--from", large},
		{"generate", "--from", small, "--overrides", large},
		{"init", "--from", small, "--overrides", large},
	} {
		var stderr bytes.Buffer
		out := filepath.Join(dir, "config_gen.go")
		args = append(args, "--out", out)
		{
			code := run(args, &stderr)
			require.Equal(t, 1, code, "%v: %d %s", args, code, &stderr)
			require.Contains(t, stderr.String(), "byte limit", "%v: %d %s", args, code, &stderr)
		}
		{
			_, err := os.Stat(out)
			require.True(t, os.IsNotExist(err), "output created: %v", err)
		}
	}
}
