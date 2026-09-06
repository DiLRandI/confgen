package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestExternalKeyConsumer(t *testing.T) {
	const input = `version: 1
package: consumer
env_prefix: APP
fields:
  server_port: {type: int, key: server-port}
  logging_level: {type: string, key: logging.level}
  http_server: {type: bool, key: HTTPServer}
  database:
    type: object
    key: databaseConfig
    fields:
      url: {type: string, key: databaseURL}
  backends:
    type: list
    default: [{backendName: primary, useTLS: true}]
    items:
      type: object
      fields:
        name: {type: string, key: backendName}
        tls: {type: bool, key: useTLS}
`
	m, err := schema.Compile("keys.schema.yaml", []byte(input))
	require.NoError(t, err)
	code, err := generator.Generate(m, generator.Options{})
	require.NoError(t, err)
	rendered, err := schema.Render(m)
	require.NoError(t, err)
	round, err := schema.Compile("round.schema.yaml", rendered)
	require.NoError(t, err)
	again, err := generator.Generate(round, generator.Options{})
	require.NoError(t, err, "contract round trip: %v", err)
	require.Equal(t, code, again, "contract round trip: %v", err)
	yamlExample, err := generator.ExampleYAML(m)
	require.NoError(t, err)
	envExample, err := generator.ExampleEnv(m)
	require.NoError(t, err)
	require.Contains(t, string(envExample), string([]byte(`APP_BACKENDS=[{"backendName":"primary","useTLS":true}]`)), string(envExample))
	dir := t.TempDir()
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	mod := "module consumer\n\ngo 1.26.0\nrequire github.com/DiLRandI/confgen v0.0.0\nrequire github.com/stretchr/testify v1.12.1\nreplace github.com/DiLRandI/confgen => " + strconvQuote(root) + "\n"
	consumer := `package consumer

import "github.com/stretchr/testify/require"
import (
	"encoding/json"
	"github.com/DiLRandI/confgen/config"
	"go.yaml.in/yaml/v3"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	for _, tc := range []struct {
		format config.Format
		input  string
	}{
		{config.FormatYAML, "server-port: 8080\nlogging.level: info\nHTTPServer: true\ndatabaseConfig: {databaseURL: local}\nbackends: [{backendName: secondary, useTLS: false}]"},
		{config.FormatJSON, "{\"server-port\":8080,\"logging.level\":\"info\",\"HTTPServer\":true,\"databaseConfig\":{\"databaseURL\":\"local\"},\"backends\":[{\"backendName\":\"secondary\",\"useTLS\":false}]}"},
	} {
		cfg, err := Load(config.Reader("input", strings.NewReader(tc.input), tc.format), config.Env(config.WithLookupEnv(func(k string) (string, bool) { return "9090", k == "APP_SERVER_PORT" })))
		require.NoError(t, err)
		require.Equal(t, 9090, cfg.ServerPort, "external values lost")
		require.Equal(t, "info", cfg.LoggingLevel, "external values lost")
		require.True(t, cfg.HTTPServer, "external values lost")
		require.Equal(t, "local", cfg.Database.URL, "external values lost")
		require.Equal(t, "secondary", cfg.Backends[0].Name, "external values lost")
		require.False(t, cfg.Backends[0].TLS, "external values lost")
		for _, marshal := range []func(any) ([]byte, error){json.Marshal, yaml.Marshal} {
			b, err := marshal(cfg)
			require.NoError(t, err)
			for _, key := range []string{"server-port", "logging.level", "HTTPServer", "databaseConfig", "databaseURL", "backendName", "useTLS"} {
				require.Contains(t, string(b), key, "missing tag %s", key)
			}
		}
	}
	cfg, err := Load()
	require.NoError(t, err, "collection default lost", err)
	require.Equal(t, 1, len(cfg.Backends), "collection default lost", err)
	require.Equal(t, "primary", cfg.Backends[0].Name, "collection default lost", err)
	require.True(t, cfg.Backends[0].TLS, "collection default lost", err)
	{
		_, err = Load(config.File("example.yaml"))
		require.NoError(t, err, "example cannot load", err)
	}
	_, err = Load(config.Reader("bad", strings.NewReader("databaseConfig: {databaseURL: 12345}"), config.FormatYAML))
	var ce *config.Error
	require.ErrorAs(t, err, &ce, "noncanonical or unsafe error: %v", err)
	require.Equal(t, "database.url", ce.Issues[0].Path, "noncanonical or unsafe error: %v", err)
	require.NotContains(t, err.Error(), "12345", "noncanonical or unsafe error: %v", err)
	_, err = Load(config.Reader("unknown", strings.NewReader("databaseConfig: {unknown: value}"), config.FormatYAML))
	require.ErrorAs(t, err, &ce, "unknown path: %v", err)
	require.Equal(t, "database.unknown", ce.Issues[0].Path, "unknown path: %v", err)
}
`
	for name, data := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "consumer_test.go": []byte(consumer), "example.yaml": yamlExample} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
	}
	cmd := exec.Command("go", "test", "-mod=mod", "-race", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	{
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "consumer: %v\n%s", err, out)
	}
}
