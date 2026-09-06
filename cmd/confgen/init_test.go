package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/schema"
)

func TestInitAndGenerate(t *testing.T) {
	for _, ext := range []string{"yaml", "yml", "json"} {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "config."+ext)
			original := []byte(`{"server":{"port":8080,"timeout":"30s"}}`)
			if err := os.WriteFile(input, original, 0o600); err != nil {
				t.Fatal(err)
			}
			schemaPath := filepath.Join(dir, "appconfig", "config.schema.yaml")
			out := filepath.Join(dir, "appconfig", "config_gen.go")
			args := []string{"init", "--from", input, "--package", "appconfig", "--schema", schemaPath, "--out", out}
			var log bytes.Buffer
			if run(args, &log) != 0 {
				t.Fatal(log.String())
			}
			b, err := os.ReadFile(schemaPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = schema.Compile(schemaPath, b); err != nil {
				t.Fatal(err)
			}
			if run(args, &log) == 0 {
				t.Fatal("init overwrote maintained schema")
			}
			unchanged, _ := os.ReadFile(input)
			if !bytes.Equal(original, unchanged) {
				t.Fatal("input changed")
			}
			generated, _ := os.ReadFile(out)
			if run([]string{"generate", "--from", input, "--package", "appconfig", "--out", out}, &log) != 0 {
				t.Fatal(log.String())
			}
			direct, _ := os.ReadFile(out)
			if !bytes.Equal(generated, direct) {
				t.Fatal("direct generation differs from init")
			}
			b = bytes.Replace(b, []byte("type: string"), []byte("type: duration"), 1)
			if err := os.WriteFile(schemaPath, b, 0o600); err != nil {
				t.Fatal(err)
			}
			if run([]string{"generate", "--schema", schemaPath, "--out", out}, &log) != 0 {
				t.Fatal(log.String())
			}
			after, _ := os.ReadFile(schemaPath)
			if !bytes.Equal(b, after) {
				t.Fatal("generate replaced edited metadata")
			}
		})
	}
}

func TestInitFailures(t *testing.T) {
	for _, tc := range []struct{ input, ext, want string }{{"x: null", "yaml", "null"}, {"x: []", "yaml", "empty"}, {"x: [1, a]", "yaml", "incompatible"}, {"x: [", "yaml", "syntax"}, {"x = 1", "toml", "extension"}} {
		dir := t.TempDir()
		input := filepath.Join(dir, "config."+tc.ext)
		if err := os.WriteFile(input, []byte(tc.input), 0o600); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "generated.go")
		var b bytes.Buffer
		if run([]string{"init", "--from", input, "--schema", filepath.Join(dir, "schema.yaml"), "--out", out}, &b) == 0 || !strings.Contains(b.String(), tc.want) {
			t.Fatal(b.String())
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatal("partial output")
		}
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "input.yaml")
	if err := os.WriteFile(input, []byte("x: 1"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init"}, {"init", "--from", filepath.Join(dir, "missing.yaml")}, {"init", "--from", input, "--schema", input}, {"init", "--from", input, "--schema", filepath.Join(dir, "same"), "--out", filepath.Join(dir, "same")}} {
		var b bytes.Buffer
		if run(args, &b) == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
	var b bytes.Buffer
	if run([]string{"init", "-h"}, &b) != 0 || !strings.Contains(b.String(), "never overwritten") || !strings.Contains(b.String(), "config.schema.yaml") || !strings.Contains(b.String(), "config_gen.go") {
		t.Fatal(b.String())
	}
}

func TestInitCopyDefaults(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "config.yaml")
	original := []byte("server: {port: 8080, host: localhost}\norigins: [a, b]\npassword: sentinel-secret\n")
	if err := os.WriteFile(input, original, 0o600); err != nil {
		t.Fatal(err)
	}
	schemaPath := filepath.Join(dir, "schema.yaml")
	out := filepath.Join(dir, "config.go")
	var stderr bytes.Buffer
	withoutSchema := filepath.Join(dir, "without-schema.yaml")
	withoutOut := filepath.Join(dir, "without.go")
	if code := run([]string{"init", "--from", input, "--schema", withoutSchema, "--out", withoutOut}, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	withoutCode, err := os.ReadFile(withoutOut)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(withoutCode, []byte("sentinel-secret")) {
		t.Fatal("init copied an input secret without opt-in")
	}
	if code := run([]string{"init", "--from", input, "--copy-defaults", "--schema", schemaPath, "--out", out}, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(schemaBytes, []byte("default:")) {
		t.Fatal("init --copy-defaults did not copy defaults")
	}
	withCode, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(withCode, []byte("sentinel-secret")) {
		t.Fatal("init --copy-defaults did not copy input values")
	}
	unchanged, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, unchanged) {
		t.Fatal("init changed input")
	}
}

func TestInitDefaultOutputPaths(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(input, []byte("port: 8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stderr bytes.Buffer
	if code := run([]string{"init", "--from", input}, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	for _, path := range []string{filepath.Join("appconfig", "config.schema.yaml"), filepath.Join("appconfig", "config_gen.go")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("default output %q: %v", path, err)
		}
	}
}

func TestInitPackageOutputPaths(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(input, []byte("port: 8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stderr bytes.Buffer
	if code := run([]string{"init", "--from", input, "--package", "settings"}, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	for _, path := range []string{filepath.Join("settings", "config.schema.yaml"), filepath.Join("settings", "config_gen.go")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("package output %q: %v", path, err)
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
			if err := os.WriteFile(input, []byte("port: 8080\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Chdir(dir)
			args := append([]string{"init", "--from", input}, tc.args...)
			var stderr bytes.Buffer
			if code := run(args, &stderr); code != 0 {
				t.Fatal(stderr.String())
			}
			for _, path := range []string{tc.expSchema, tc.expOut} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("output %q: %v", path, err)
				}
			}
		})
	}
}

func TestInitInvalidPackageDoesNotCreateDefaultDirectory(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.yaml")
	if err := os.WriteFile(input, []byte("port: 8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stderr bytes.Buffer
	if code := run([]string{"init", "--from", input, "--package", "bad-package"}, &stderr); code == 0 {
		t.Fatal("accepted invalid package")
	}
	if _, err := os.Stat(filepath.Join(dir, "bad-package")); !os.IsNotExist(err) {
		t.Fatalf("created output directory before package validation: %v", err)
	}
}

func TestInitExplicitEmptyOutputPaths(t *testing.T) {
	for _, flagName := range []string{"--schema", "--out"} {
		t.Run(flagName, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "input.yaml")
			if err := os.WriteFile(input, []byte("port: 8080\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Chdir(dir)
			var stderr bytes.Buffer
			if code := run([]string{"init", "--from", input, flagName + "="}, &stderr); code == 0 {
				t.Fatal("accepted explicitly empty output path")
			}
			if _, err := os.Stat(filepath.Join(dir, "appconfig")); !os.IsNotExist(err) {
				t.Fatalf("created output directory after rejection: %v", err)
			}
		})
	}
}

func TestInitExplicitEmptyPackage(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.yaml")
	if err := os.WriteFile(input, []byte("port: 8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	var stderr bytes.Buffer
	if code := run([]string{"init", "--from", input, "--package="}, &stderr); code == 0 {
		t.Fatal("accepted explicitly empty package")
	}
	if _, err := os.Stat("config.schema.yaml"); !os.IsNotExist(err) {
		t.Fatalf("created output after empty package rejection: %v", err)
	}
}
