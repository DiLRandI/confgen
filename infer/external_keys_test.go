package infer_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
)

func TestInferExternalKeysConsumer(t *testing.T) {
	for _, tc := range []struct{ ext, input string }{
		{"yaml", "server-port: 8080\ndatabaseURL: local\nlogging.level: info\nHTTPServer: {bindHost: localhost}\nbackends: [{backendName: a}, {useTLS: true}]\n"},
		{"json", `{"server-port":8080,"databaseURL":"local","logging.level":"info","HTTPServer":{"bindHost":"localhost"},"backends":[{"backendName":"a"},{"useTLS":true}]}`},
	} {
		t.Run(tc.ext, func(t *testing.T) {
			for _, copyDefaults := range []bool{false, true} {
				m, err := infer.FromConfig("config."+tc.ext, []byte(tc.input), infer.Options{Package: "consumer", CopyDefaults: copyDefaults})
				if err != nil {
					t.Fatal(err)
				}
				b, err := schema.Render(m)
				if err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{"server_port:", "key: server-port", "database_url:", "key: databaseURL", "logging_level:", "key: logging.level", "http_server:", "key: HTTPServer", "bind_host:"} {
					if !bytes.Contains(b, []byte(want)) {
						t.Fatalf("missing %s\n%s", want, b)
					}
				}
				round, err := schema.Compile("round.yaml", b)
				if err != nil {
					t.Fatal(err)
				}
				code, err := generator.Generate(round, generator.Options{})
				if err != nil {
					t.Fatal(err)
				}
				again, err := generator.Generate(m, generator.Options{})
				if err != nil || !bytes.Equal(code, again) {
					t.Fatal("roundtrip changed code", err)
				}
				dir := t.TempDir()
				root, err := filepath.Abs("..")
				if err != nil {
					t.Fatal(err)
				}
				mod := "module consumer\n\ngo 1.26.0\nrequire github.com/DiLRandI/confgen v0.0.0\nreplace github.com/DiLRandI/confgen => " + strconv.Quote(root) + "\n"
				consumer := `package consumer
import("testing";"github.com/DiLRandI/confgen/config")
func TestOriginal(t *testing.T){
 c,e:=Load(config.File("config.` + tc.ext + `"));if e!=nil{t.Fatal(e)}
 if c.ServerPort!=8080||c.DatabaseURL!="local"||c.LoggingLevel!="info"||c.HTTPServer.BindHost!="localhost"||len(c.Backends)!=2{t.Fatal("original values lost")}
 if c.Backends[0].BackendName!="a"||!c.Backends[1].UseTLS{t.Fatal("list fields lost")}
}
`
				for name, data := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "config." + tc.ext: []byte(tc.input), "consumer_test.go": []byte(consumer)} {
					if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				cmd := exec.Command("go", "test", "-mod=mod", "./...")
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOWORK=off")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("consumer %v\n%s", err, out)
				}
				original, err := os.ReadFile(filepath.Join(dir, "config."+tc.ext))
				if err != nil || string(original) != tc.input {
					t.Fatal("source changed", err)
				}
			}
		})
	}
}

func TestExternalKeyCanonicalCollisions(t *testing.T) {
	for _, tc := range []struct{ ext, input string }{
		{"yaml", "server-port: 1\nserver_port: 2"},
		{"json", `{"server-port":1,"server_port":2}`},
		{"yaml", "backends: [{server-port: 1}, {server_port: 2}]"},
		{"json", `{"backends":[{"server-port":1},{"server_port":2}]}`},
	} {
		_, err := infer.FromConfig("config."+tc.ext, []byte(tc.input), infer.Options{})
		if err == nil {
			t.Fatal("collision accepted")
		}
		for _, want := range []string{`"server-port"`, `"server_port"`, "canonical"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("missing %s: %v", want, err)
			}
		}
	}
}
