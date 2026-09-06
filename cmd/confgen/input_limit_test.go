package main

import (
	"bytes"
	"github.com/DiLRandI/confgen/input"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIInputLimits(t *testing.T) {
	dir := t.TempDir()
	large := filepath.Join(dir, "large.yaml")
	f, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(int64(input.DefaultLimit) + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	small := filepath.Join(dir, "small.yaml")
	if err := os.WriteFile(small, []byte("value: hello\n"), 0600); err != nil {
		t.Fatal(err)
	}
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
		if code := run(args, &stderr); code != 1 || !strings.Contains(stderr.String(), "byte limit") {
			t.Fatalf("%v: %d %s", args, code, &stderr)
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatalf("output created: %v", err)
		}
	}
}
