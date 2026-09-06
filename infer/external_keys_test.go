package infer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestInferExternalKeysConsumer(t *testing.T) {
	for _, tc := range []struct{ ext, input string }{
		{"yaml", "server-port: 8080\ndatabaseURL: local\nlogging.level: info\nHTTPServer: {bindHost: localhost}\nbackends: [{backendName: a}, {useTLS: true}]\n"},
		{"json", `{"server-port":8080,"databaseURL":"local","logging.level":"info","HTTPServer":{"bindHost":"localhost"},"backends":[{"backendName":"a"},{"useTLS":true}]}`},
	} {
		t.Run(tc.ext, func(t *testing.T) {
			for _, copyDefaults := range []bool{false, true} {
				m, err := infer.FromConfig("config."+tc.ext, []byte(tc.input), infer.Options{Package: "consumer", CopyDefaults: copyDefaults})
				require.NoError(t, err)
				b, err := schema.Render(m)
				require.NoError(t, err)
				for _, want := range []string{"server_port:", "key: server-port", "database_url:", "key: databaseURL", "logging_level:", "key: logging.level", "http_server:", "key: HTTPServer", "bind_host:"} {
					require.Contains(t, string(b), want)
				}
				round, err := schema.Compile("round.yaml", b)
				require.NoError(t, err)
				code, err := generator.Generate(round, generator.Options{})
				require.NoError(t, err)
				again, err := generator.Generate(m, generator.Options{})
				require.NoError(t, err)
				require.Equal(t, code, again)
				dir := t.TempDir()
				root, err := filepath.Abs("..")
				require.NoError(t, err)
				mod := "module consumer\n\ngo 1.26.0\nrequire github.com/DiLRandI/confgen v0.0.0\nrequire github.com/stretchr/testify v1.12.1\nreplace github.com/DiLRandI/confgen => " + strconv.Quote(root) + "\n"
				consumer := `package consumer
import("testing";"github.com/DiLRandI/confgen/config";"github.com/stretchr/testify/require")
func TestOriginal(t *testing.T){
 c,e:=Load(config.File("config.` + tc.ext + `"))
 require.NoError(t,e)
 require.Equal(t,int64(8080),c.ServerPort)
 require.Equal(t,"local",c.DatabaseURL)
 require.Equal(t,"info",c.LoggingLevel)
 require.Equal(t,"localhost",c.HTTPServer.BindHost)
 require.Len(t,c.Backends,2)
 require.Equal(t,"a",c.Backends[0].BackendName)
 require.True(t,c.Backends[1].UseTLS)
}
`
				for name, data := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "config." + tc.ext: []byte(tc.input), "consumer_test.go": []byte(consumer)} {
					err := os.WriteFile(filepath.Join(dir, name), data, 0o600)
					require.NoError(t, err)
				}
				cmd := exec.Command("go", "test", "-mod=mod", "./...")
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOWORK=off")
				out, err := cmd.CombinedOutput()
				require.NoError(t, err, "consumer output: %s", out)
				original, err := os.ReadFile(filepath.Join(dir, "config."+tc.ext))
				require.NoError(t, err)
				require.Equal(t, tc.input, string(original))
			}
		})
	}
}

func TestExternalKeyCanonicalCollisions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ ext, input string }{
		{"yaml", "server-port: 1\nserver_port: 2"},
		{"json", `{"server-port":1,"server_port":2}`},
		{"yaml", "backends: [{server-port: 1}, {server_port: 2}]"},
		{"json", `{"backends":[{"server-port":1},{"server_port":2}]}`},
	} {
		_, err := infer.FromConfig("config."+tc.ext, []byte(tc.input), infer.Options{})
		require.Error(t, err)
		for _, want := range []string{`"server-port"`, `"server_port"`, "canonical"} {
			require.Contains(t, err.Error(), want)
		}
	}
}
