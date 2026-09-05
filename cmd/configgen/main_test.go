package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.yaml")
	out := filepath.Join(dir, "config_gen.go")
	if e := os.WriteFile(schema, []byte("version: 1\npackage: app\nfields: {port: {type: int, default: 8080}}"), 0600); e != nil {
		t.Fatal(e)
	}
	var stderr bytes.Buffer
	if code := run([]string{"-schema", schema, "-out", out, "-example-yaml", filepath.Join(dir, "example.yaml"), "-example-env", filepath.Join(dir, "env.example")}, &stderr); code != 0 {
		t.Fatalf("%d %s", code, stderr.String())
	}
	before, _ := os.ReadFile(out)
	for _, args := range [][]string{{}, {"-schema", schema, "-out", schema}, {"-schema", schema, "-out", out, "-example-env", out}, {"-unknown"}} {
		stderr.Reset()
		if code := run(args, &stderr); code == 0 || stderr.Len() == 0 {
			t.Fatalf("accepted %v", args)
		}
	}
	if e := os.WriteFile(schema, []byte("version: 1\npackage: app\nfields: {x: {type: duration, default: wrong}}"), 0600); e != nil {
		t.Fatal(e)
	}
	stderr.Reset()
	if run([]string{"-schema", schema, "-out", out}, &stderr) == 0 || strings.Contains(stderr.String(), "panic") {
		t.Fatal("bad error behavior")
	}
	after, _ := os.ReadFile(out)
	if !bytes.Equal(before, after) {
		t.Fatal("failed schema replaced output")
	}
}
