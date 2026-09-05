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
	if run([]string{"init", "-h"}, &b) != 0 || !strings.Contains(b.String(), "never overwritten") {
		t.Fatal(b.String())
	}
}
