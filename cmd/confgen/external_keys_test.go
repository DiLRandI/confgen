package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitExternalKeys(t *testing.T) {
	for _, tc := range []struct{ ext, input string }{
		{"yaml", "server-port: 8080\ndatabaseURL: local\nlogging.level: info\n"},
		{"json", `{"server-port":8080,"databaseURL":"local","logging.level":"info"}`},
	} {
		t.Run(tc.ext, func(t *testing.T) {
			t.Chdir(t.TempDir())
			input := "config." + tc.ext
			require.NoError(t, os.WriteFile(input, []byte(tc.input), 0o600))
			var stderr bytes.Buffer
			require.Equal(t, 0, run([]string{"init", "--from", input, "--package", "appconfig"}, &stderr), stderr.String())
			b, err := os.ReadFile(filepath.Join("appconfig", "config_gen.go"))
			require.NoError(t, err)
			for _, key := range []string{`json:"server-port"`, `json:"databaseURL"`, `json:"logging.level"`} {
				require.Contains(t, string(b), string([]byte(key)), "missing %s", key)
			}
			unchanged, err := os.ReadFile(input)
			require.NoError(t, err, "source changed", err)
			require.Equal(t, tc.input, string(unchanged), "source changed", err)
		})
	}
}
