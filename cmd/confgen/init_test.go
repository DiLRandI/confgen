package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestInitAndGenerate(t *testing.T) {
	for _, ext := range []string{"yaml", "yml", "json"} {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "config."+ext)
			original := []byte(`{"server":{"port":8080,"timeout":"30s"}}`)
			require.NoError(t, os.WriteFile(input, original, 0o600))
			schemaPath := filepath.Join(dir, "appconfig", "config.schema.yaml")
			out := filepath.Join(dir, "appconfig", "config_gen.go")
			args := []string{"init", "--from", input, "--package", "appconfig", "--schema", schemaPath, "--out", out}
			var log bytes.Buffer
			require.Equal(t, 0, run(args, &log), log.String())
			b, err := os.ReadFile(schemaPath)
			require.NoError(t, err)
			{
				_, err = schema.Compile(schemaPath, b)
				require.NoError(t, err)
			}
			require.NotEqual(t, 0, run(args, &log), "init overwrote maintained schema")
			unchanged, _ := os.ReadFile(input)
			require.Equal(t, original, unchanged, "input changed")
			generated, _ := os.ReadFile(out)
			require.Equal(t, 0, run([]string{"generate", "--from", input, "--package", "appconfig", "--out", out}, &log), log.String())
			direct, _ := os.ReadFile(out)
			require.Equal(t, generated, direct, "direct generation differs from init")
			b = bytes.Replace(b, []byte("type: string"), []byte("type: duration"), 1)
			require.NoError(t, os.WriteFile(schemaPath, b, 0o600))
			require.Equal(t, 0, run([]string{"generate", "--schema", schemaPath, "--out", out}, &log), log.String())
			after, _ := os.ReadFile(schemaPath)
			require.Equal(t, b, after, "generate replaced edited metadata")
		})
	}
}

func TestInitFailures(t *testing.T) {
	for _, tc := range []struct{ input, ext, want string }{{"x: null", "yaml", "null"}, {"x: []", "yaml", "empty"}, {"x: [1, a]", "yaml", "incompatible"}, {"x: [", "yaml", "syntax"}, {"x = 1", "toml", "extension"}} {
		dir := t.TempDir()
		input := filepath.Join(dir, "config."+tc.ext)
		require.NoError(t, os.WriteFile(input, []byte(tc.input), 0o600))
		out := filepath.Join(dir, "generated.go")
		var b bytes.Buffer
		require.NotEqual(t, 0, run([]string{"init", "--from", input, "--schema", filepath.Join(dir, "schema.yaml"), "--out", out}, &b), b.String())
		require.Contains(t, b.String(), tc.want, b.String())
		{
			_, err := os.Stat(out)
			require.True(t, os.IsNotExist(err), "partial output")
		}
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "input.yaml")
	require.NoError(t, os.WriteFile(input, []byte("x: 1"), 0o600))
	for _, args := range [][]string{{"init"}, {"init", "--from", filepath.Join(dir, "missing.yaml")}, {"init", "--from", input, "--schema", input}, {"init", "--from", input, "--schema", filepath.Join(dir, "same"), "--out", filepath.Join(dir, "same")}} {
		var b bytes.Buffer
		require.NotEqual(t, 0, run(args, &b), "accepted %v", args)
	}
	var b bytes.Buffer
	require.Equal(t, 0, run([]string{"init", "-h"}, &b), b.String())
	require.Contains(t, b.String(), "never overwritten", b.String())
	require.Contains(t, b.String(), "config.schema.yaml", b.String())
	require.Contains(t, b.String(), "config_gen.go", b.String())
}

func TestInitCopyDefaults(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "config.yaml")
	original := []byte("server: {port: 8080, host: localhost}\norigins: [a, b]\npassword: sentinel-secret\n")
	require.NoError(t, os.WriteFile(input, original, 0o600))
	schemaPath := filepath.Join(dir, "schema.yaml")
	out := filepath.Join(dir, "config.go")
	var stderr bytes.Buffer
	withoutSchema := filepath.Join(dir, "without-schema.yaml")
	withoutOut := filepath.Join(dir, "without.go")
	require.Equal(t, 0, run([]string{"init", "--from", input, "--schema", withoutSchema, "--out", withoutOut}, &stderr), stderr.String())
	withoutCode, err := os.ReadFile(withoutOut)
	require.NoError(t, err)
	require.NotContains(t, string(withoutCode), "sentinel-secret", "init copied an input secret without opt-in")
	require.Equal(t, 0, run([]string{"init", "--from", input, "--copy-defaults", "--schema", schemaPath, "--out", out}, &stderr), stderr.String())
	schemaBytes, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	require.Contains(t, string(schemaBytes), "default:", "init --copy-defaults did not copy defaults")
	withCode, err := os.ReadFile(out)
	require.NoError(t, err)
	require.Contains(t, string(withCode), "sentinel-secret", "init --copy-defaults did not copy input values")
	unchanged, err := os.ReadFile(input)
	require.NoError(t, err)
	require.Equal(t, original, unchanged, "init changed input")
}

func TestInitDefaultOutputPaths(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(input, []byte("port: 8080\n"), 0o600))
	t.Chdir(dir)
	var stderr bytes.Buffer
	require.Equal(t, 0, run([]string{"init", "--from", input}, &stderr), stderr.String())
	for _, path := range []string{filepath.Join("appconfig", "config.schema.yaml"), filepath.Join("appconfig", "config_gen.go")} {
		{
			_, err := os.Stat(path)
			require.NoError(t, err, "default output %q: %v", path, err)
		}
	}
}

func TestInitPackageOutputPaths(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(input, []byte("port: 8080\n"), 0o600))
	t.Chdir(dir)
	var stderr bytes.Buffer
	require.Equal(t, 0, run([]string{"init", "--from", input, "--package", "settings"}, &stderr), stderr.String())
	for _, path := range []string{filepath.Join("settings", "config.schema.yaml"), filepath.Join("settings", "config_gen.go")} {
		{
			_, err := os.Stat(path)
			require.NoError(t, err, "package output %q: %v", path, err)
		}
	}
}

func TestInitMixedOutputPaths(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		expSchema string
		expOut    string
	}{
		{name: "schema only", args: []string{"--schema", "custom/schema.yaml"}, expSchema: "custom/schema.yaml", expOut: filepath.Join("appconfig", "config_gen.go")},
		{name: "out only", args: []string{"--out", "custom/config.go"}, expSchema: filepath.Join("appconfig", "config.schema.yaml"), expOut: "custom/config.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input.yaml")
			require.NoError(t, os.WriteFile(input, []byte("port: 8080\n"), 0o600))
			t.Chdir(dir)
			args := append([]string{"init", "--from", input}, tc.args...)
			var stderr bytes.Buffer
			require.Equal(t, 0, run(args, &stderr), stderr.String())
			for _, path := range []string{tc.expSchema, tc.expOut} {
				{
					_, err := os.Stat(path)
					require.NoError(t, err, "output %q: %v", path, err)
				}
			}
		})
	}
}

func TestInitInvalidPackageDoesNotCreateDefaultDirectory(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.yaml")
	require.NoError(t, os.WriteFile(input, []byte("port: 8080\n"), 0o600))
	t.Chdir(dir)
	var stderr bytes.Buffer
	require.NotEqual(t, 0, run([]string{"init", "--from", input, "--package", "bad-package"}, &stderr), "accepted invalid package")
	{
		_, err := os.Stat(filepath.Join(dir, "bad-package"))
		require.True(t, os.IsNotExist(err), "created output directory before package validation: %v", err)
	}
}

func TestInitExplicitEmptyOutputPaths(t *testing.T) {
	for _, flagName := range []string{"--schema", "--out"} {
		t.Run(flagName, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input.yaml")
			require.NoError(t, os.WriteFile(input, []byte("port: 8080\n"), 0o600))
			t.Chdir(dir)
			var stderr bytes.Buffer
			require.NotEqual(t, 0, run([]string{"init", "--from", input, flagName + "="}, &stderr), "accepted explicitly empty output path")
			{
				_, err := os.Stat(filepath.Join(dir, "appconfig"))
				require.True(t, os.IsNotExist(err), "created output directory after rejection: %v", err)
			}
		})
	}
}

func TestInitExplicitEmptyPackage(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.yaml")
	require.NoError(t, os.WriteFile(input, []byte("port: 8080\n"), 0o600))
	t.Chdir(dir)
	var stderr bytes.Buffer
	require.NotEqual(t, 0, run([]string{"init", "--from", input, "--package="}, &stderr), "accepted explicitly empty package")
	{
		_, err := os.Stat("config.schema.yaml")
		require.True(t, os.IsNotExist(err), "created output after empty package rejection: %v", err)
	}
}
