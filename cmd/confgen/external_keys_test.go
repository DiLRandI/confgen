package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInitExternalKeys(t *testing.T) {
	for _, tc := range []struct{ ext, input string }{
		{"yaml", "server-port: 8080\ndatabaseURL: local\nlogging.level: info\n"},
		{"json", `{"server-port":8080,"databaseURL":"local","logging.level":"info"}`},
	} {
		t.Run(tc.ext, func(t *testing.T) {
			t.Chdir(t.TempDir())
			input := "config." + tc.ext
			if err := os.WriteFile(input, []byte(tc.input), 0o600); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			if run([]string{"init", "--from", input, "--package", "appconfig"}, &stderr) != 0 {
				t.Fatal(stderr.String())
			}
			b, err := os.ReadFile(filepath.Join("appconfig", "config_gen.go"))
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{`json:"server-port"`, `json:"databaseURL"`, `json:"logging.level"`} {
				if !bytes.Contains(b, []byte(key)) {
					t.Fatalf("missing %s", key)
				}
			}
			unchanged, err := os.ReadFile(input)
			if err != nil || string(unchanged) != tc.input {
				t.Fatal("source changed", err)
			}
		})
	}
}
