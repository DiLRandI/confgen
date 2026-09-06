package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIOverrides(t *testing.T) {
	for _, ext := range []string{"yaml", "json"} {
		t.Run(ext, func(t *testing.T) {
			t.Chdir(t.TempDir())
			input := `{"databaseURL":null,"allowed-hosts":[],"server":{"timeout":"30s"}}`
			rules := "fields:\n  database_url: {type: string}\n  allowed_hosts: {type: list, items: {type: string}}\n  server.timeout: {type: duration}\n"
			for name, data := range map[string]string{"config." + ext: input, "overrides.yaml": rules} {
				if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var stderr bytes.Buffer
			if run([]string{"init", "--from", "config." + ext, "--overrides", "overrides.yaml"}, &stderr) != 0 {
				t.Fatal(stderr.String())
			}
			generated, err := os.ReadFile(filepath.Join("appconfig", "config_gen.go"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(generated, []byte("configTime.Duration")) {
				t.Fatal("duration not generated")
			}
			if run([]string{"generate", "--from", "config." + ext, "--overrides", "overrides.yaml", "--out", "direct.go"}, &stderr) != 0 {
				t.Fatal(stderr.String())
			}
			direct, err := os.ReadFile("direct.go")
			if err != nil || !bytes.Equal(generated, direct) {
				t.Fatal("pipelines differ", err)
			}
			for _, args := range [][]string{
				{"generate", "--from", "config." + ext, "--overrides", "overrides.yaml", "--out", "overrides.yaml"},
				{"init", "--from", "config." + ext, "--overrides", "overrides.yaml", "--schema", "overrides.yaml"},
				{"init", "--from", "config." + ext, "--overrides", "missing.yaml"},
				{"generate", "--schema", "appconfig/config.schema.yaml", "--overrides", "overrides.yaml"},
			} {
				if run(args, &stderr) == 0 {
					t.Fatal("accepted unsafe or invalid arguments", args)
				}
			}
			for name, want := range map[string]string{"config." + ext: input, "overrides.yaml": rules} {
				b, err := os.ReadFile(name)
				if err != nil || string(b) != want {
					t.Fatal("input changed", name, err)
				}
			}
		})
	}
}

func TestCLIInvalidOverridesNoOutputs(t *testing.T) {
	for _, rules := range []string{"fields: {unknown: {type: string}}", "fields: {origins: {type: list}}", "fields: {origins: {type: string}}", "fields: {origins: {type: list, secret: true}}"} {
		t.Run(rules, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("config.yaml", []byte("origins: []"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("overrides.yaml", []byte(rules), 0o600); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			if run([]string{"init", "--from", "config.yaml", "--overrides", "overrides.yaml"}, &stderr) == 0 {
				t.Fatal("accepted invalid override")
			}
			if _, err := os.Stat("appconfig"); !os.IsNotExist(err) {
				t.Fatal("output created on failure", err)
			}
		})
	}
	var help bytes.Buffer
	if run([]string{"init", "-h"}, &help) != 0 || !strings.Contains(help.String(), "overrides") {
		t.Fatal(help.String())
	}
}
