package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCLIOverrides(t *testing.T) {
	for _, ext := range []string{"yaml", "json"} {
		t.Run(ext, func(t *testing.T) {
			t.Chdir(t.TempDir())
			input := `{"databaseURL":null,"allowed-hosts":[],"server":{"timeout":"30s"}}`
			rules := "fields:\n  database_url: {type: string}\n  allowed_hosts: {type: list, items: {type: string}}\n  server.timeout: {type: duration}\n"
			for name, data := range map[string]string{"config." + ext: input, "overrides.yaml": rules} {
				require.NoError(t, os.WriteFile(name, []byte(data), 0o600))
			}
			var stderr bytes.Buffer
			require.Equal(t, 0, run([]string{"init", "--from", "config." + ext, "--overrides", "overrides.yaml"}, &stderr), stderr.String())
			generated, err := os.ReadFile(filepath.Join("appconfig", "config_gen.go"))
			require.NoError(t, err)
			require.Contains(t, string(generated), "configTime.Duration", "duration not generated")
			require.Equal(t, 0, run([]string{"generate", "--from", "config." + ext, "--overrides", "overrides.yaml", "--out", "direct.go"}, &stderr), stderr.String())
			direct, err := os.ReadFile("direct.go")
			require.NoError(t, err, "pipelines differ", err)
			require.Equal(t, generated, direct, "pipelines differ", err)
			for _, args := range [][]string{
				{"generate", "--from", "config." + ext, "--overrides", "overrides.yaml", "--out", "overrides.yaml"},
				{"init", "--from", "config." + ext, "--overrides", "overrides.yaml", "--schema", "overrides.yaml"},
				{"init", "--from", "config." + ext, "--overrides", "missing.yaml"},
				{"generate", "--schema", "appconfig/config.schema.yaml", "--overrides", "overrides.yaml"},
			} {
				require.NotEqual(t, 0, run(args, &stderr), "accepted unsafe or invalid arguments", args)
			}
			for name, want := range map[string]string{"config." + ext: input, "overrides.yaml": rules} {
				b, err := os.ReadFile(name)
				require.NoError(t, err, "input changed", name, err)
				require.Equal(t, want, string(b), "input changed", name, err)
			}
		})
	}
}

func TestCLIInvalidOverridesNoOutputs(t *testing.T) {
	for _, rules := range []string{"fields: {unknown: {type: string}}", "fields: {origins: {type: list}}", "fields: {origins: {type: string}}", "fields: {origins: {type: list, secret: true}}"} {
		t.Run(rules, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("config.yaml", []byte("origins: []"), 0o600))
			require.NoError(t, os.WriteFile("overrides.yaml", []byte(rules), 0o600))
			var stderr bytes.Buffer
			require.NotEqual(t, 0, run([]string{"init", "--from", "config.yaml", "--overrides", "overrides.yaml"}, &stderr), "accepted invalid override")
			{
				_, err := os.Stat("appconfig")
				require.True(t, os.IsNotExist(err), "output created on failure", err)
			}
		})
	}
	var help bytes.Buffer
	require.Equal(t, 0, run([]string{"init", "-h"}, &help), help.String())
	require.Contains(t, help.String(), "overrides", help.String())
}
