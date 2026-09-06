package generator_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/schema"
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
	if err != nil {
		t.Fatal(err)
	}
	code, err := generator.Generate(m, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := schema.Render(m)
	if err != nil {
		t.Fatal(err)
	}
	round, err := schema.Compile("round.schema.yaml", rendered)
	if err != nil {
		t.Fatal(err)
	}
	again, err := generator.Generate(round, generator.Options{})
	if err != nil || !bytes.Equal(code, again) {
		t.Fatalf("contract round trip: %v", err)
	}
	yamlExample, err := generator.ExampleYAML(m)
	if err != nil {
		t.Fatal(err)
	}
	envExample, err := generator.ExampleEnv(m)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(envExample, []byte(`APP_BACKENDS=[{"backendName":"primary","useTLS":true}]`)) {
		t.Fatal(string(envExample))
	}
	dir := t.TempDir()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	mod := "module consumer\n\ngo 1.26.0\nrequire github.com/DiLRandI/confgen v0.0.0\nreplace github.com/DiLRandI/confgen => " + strconvQuote(root) + "\n"
	consumer := `package consumer
import (
 "encoding/json"
 "errors"
 "strings"
 "testing"
 "github.com/DiLRandI/confgen/config"
 "gopkg.in/yaml.v3"
)
func TestLoad(t *testing.T) {
 for _, tc := range []struct{format config.Format; input string}{
  {config.FormatYAML, "server-port: 8080\nlogging.level: info\nHTTPServer: true\ndatabaseConfig: {databaseURL: local}\nbackends: [{backendName: secondary, useTLS: false}]"},
  {config.FormatJSON, "{\"server-port\":8080,\"logging.level\":\"info\",\"HTTPServer\":true,\"databaseConfig\":{\"databaseURL\":\"local\"},\"backends\":[{\"backendName\":\"secondary\",\"useTLS\":false}]}"},
 } {
  cfg,err:=Load(config.Reader("input",strings.NewReader(tc.input),tc.format),config.Env(config.WithLookupEnv(func(k string)(string,bool){return "9090",k=="APP_SERVER_PORT"})))
  if err!=nil {t.Fatal(err)}
  if cfg.ServerPort!=9090||cfg.LoggingLevel!="info"||!cfg.HTTPServer||cfg.Database.URL!="local"||cfg.Backends[0].Name!="secondary"||cfg.Backends[0].TLS {t.Fatal("external values lost")}
  for _, marshal:=range []func(any)([]byte,error){json.Marshal,yaml.Marshal} {
   b,err:=marshal(cfg);if err!=nil{t.Fatal(err)}
   for _, key:=range []string{"server-port","logging.level","HTTPServer","databaseConfig","databaseURL","backendName","useTLS"}{if !strings.Contains(string(b),key){t.Fatalf("missing tag %s",key)}}
  }
 }
 cfg,err:=Load();if err!=nil||len(cfg.Backends)!=1||cfg.Backends[0].Name!="primary"||!cfg.Backends[0].TLS{t.Fatal("collection default lost",err)}
 if _,err=Load(config.File("example.yaml"));err!=nil{t.Fatal("example cannot load",err)}
 _,err=Load(config.Reader("bad",strings.NewReader("databaseConfig: {databaseURL: 12345}"),config.FormatYAML))
 var ce *config.Error
 if !errors.As(err,&ce)||ce.Issues[0].Path!="database.url"||strings.Contains(err.Error(),"12345"){t.Fatalf("noncanonical or unsafe error: %v",err)}
 _,err=Load(config.Reader("unknown",strings.NewReader("databaseConfig: {unknown: value}"),config.FormatYAML))
 if !errors.As(err,&ce)||ce.Issues[0].Path!="database.unknown"{t.Fatalf("unknown path: %v",err)}
}
`
	for name, data := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "consumer_test.go": []byte(consumer), "example.yaml": yamlExample} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-mod=mod", "-race", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("consumer: %v\n%s", err, out)
	}
}
