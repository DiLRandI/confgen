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
	if e := os.WriteFile(schema, []byte("version: 1\npackage: app\nfields: {port: {type: int, default: 8080}}"), 0o600); e != nil {
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
	if e := os.WriteFile(schema, []byte("version: 1\npackage: app\nfields: {x: {type: duration, default: wrong}}"), 0o600); e != nil {
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

func TestHelp(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"generate", "-h"}} {
		var b bytes.Buffer
		if run(args, &b) != 0 || !strings.Contains(b.String(), "confgen generate") || !strings.Contains(b.String(), "-schema") {
			t.Fatal(b.String())
		}
	}
}

func TestGenerateFromDoesNotCopyDefaultsUnlessRequested(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "config.yaml")
	original := []byte("server: {port: 8080, host: localhost}\norigins: [a, b]\npassword: sentinel-secret\n")
	if err := os.WriteFile(input, original, 0o600); err != nil {
		t.Fatal(err)
	}
	without := filepath.Join(dir, "without.go")
	with := filepath.Join(dir, "with.go")
	var stderr bytes.Buffer
	if code := run([]string{"generate", "--from", input, "--out", without}, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	if code := run([]string{"generate", "--from", input, "--copy-defaults", "--out", with}, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	withoutCode, err := os.ReadFile(without)
	if err != nil {
		t.Fatal(err)
	}
	withCode, err := os.ReadFile(with)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(withoutCode, []byte("HasDefault: true")) || !bytes.Contains(withCode, []byte("HasDefault: true")) {
		t.Fatal("generate --from default policy is incorrect")
	}
	if bytes.Contains(withoutCode, []byte("sentinel-secret")) || !bytes.Contains(withCode, []byte("sentinel-secret")) {
		t.Fatal("generate --from copied an input secret without opt-in")
	}
	unchanged, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, unchanged) {
		t.Fatal("generate --from changed input")
	}
}
