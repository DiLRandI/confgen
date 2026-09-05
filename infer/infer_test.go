package infer_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
)

func TestInference(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"config.yaml", "server: {port: 8080, host: localhost, timeout: 30s}\ndebug: false\nhuge: 18446744073709551615\nratio: 0.75\nbackends: [{name: a, port: 1}, {port: 2, name: b}]\norigins: [a, b]\nempty: {}\n"},
		{"config.json", `{"server":{"port":8080,"host":"localhost","timeout":"30s"},"debug":false,"huge":18446744073709551615,"ratio":0.75,"backends":[{"name":"a","port":1},{"port":2,"name":"b"}],"origins":["a","b"],"empty":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := infer.FromConfig(tc.name, []byte(tc.input), infer.Options{Package: "appconfig"})
			if err != nil {
				t.Fatal(err)
			}
			fields := m.Descriptor.Fields
			if fields[0].Name != "server" || fields[0].Children[0].Name != "port" || fields[0].Children[0].Kind != config.KindInt64 || fields[0].Children[2].Kind != config.KindString || fields[2].Kind != config.KindUint64 || fields[3].Kind != config.KindFloat64 {
				t.Fatalf("unexpected fields: %+v", fields)
			}
			if !fields[1].HasDefault || fields[1].Default != false || !fields[4].HasDefault || fields[4].Item.Children[0].HasDefault {
				t.Fatal("incorrect default policy")
			}
			first, err := schema.Render(m)
			if err != nil {
				t.Fatal(err)
			}
			second, err := schema.Render(m)
			if err != nil || !bytes.Equal(first, second) {
				t.Fatal("unstable schema")
			}
			compiled, err := schema.Compile("bootstrap.yaml", first)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = generator.Generate(compiled, generator.Options{}); err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(first, []byte("required:")) || bytes.Contains(first, []byte("secret:")) || bytes.Contains(first, []byte("description:")) {
				t.Fatal("invented metadata")
			}
		})
	}
}

func TestInferenceErrors(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"a.yaml", "password: null", "password"}, {"a.json", `{"password":null}`, "password"},
		{"a.yml", "values: []", "list is empty"}, {"a.json", `{"values":[]}`, "list is empty"},
		{"a.yaml", "values: [1, hello]", "item 2 is string"}, {"a.json", `{"values":[1,"hello"]}`, "item 2 is string"},
		{"a.yaml", "values: [{a: 1}, {b: 2}]", "incompatible"},
		{"a.yaml", "values: [a, null]", "values[2]"},
		{"a.yaml", "values: [[1]]", "nested lists"},
		{"a.yaml", "x: 1\nx: 2", "duplicate"}, {"a.json", `{"x":1,"x":2}`, "duplicate"},
		{"a.yaml", "x: &x [1]\ny: *x", "syntax"}, {"a.yaml", "x: {<<: {a: 1}}", "syntax"},
		{"a.yaml", "x: .nan", "finite"}, {"a.json", `{"x":1e999}`, "finite"},
		{"a.yaml", "x: 18446744073709551616", "64-bit"}, {"a.json", `{"x":18446744073709551616}`, "64-bit"},
		{"a.yaml", "x: -9223372036854775809", "64-bit"},
		{"a.yaml", "[1, 2]", "root"}, {"a.yaml", "x: [", "syntax"},
		{"a.toml", "x = 1", "extension"},
	} {
		t.Run(tc.name+tc.want, func(t *testing.T) {
			_, err := infer.FromConfig(tc.name, []byte(tc.input), infer.Options{Package: "app"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q: %v", tc.want, err)
			}
		})
	}
}

func TestRoundTripConsumer(t *testing.T) {
	for _, name := range []string{"sample.yaml", "sample.json"} {
		t.Run(name, func(t *testing.T) {
			input := `{"server":{"port":8080,"timeout":"30s"},"max":9223372036854775807,"backends":[{"name":"a","port":1},{"name":"b","port":2}]}`
			m, err := infer.FromConfig(name, []byte(input), infer.Options{Package: "consumer"})
			if err != nil {
				t.Fatal(err)
			}
			yamlBytes, err := schema.Render(m)
			if err != nil {
				t.Fatal(err)
			}
			m, err = schema.Compile("schema.yaml", yamlBytes)
			if err != nil {
				t.Fatal(err)
			}
			code, err := generator.Generate(m, generator.Options{})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			root, err := filepath.Abs("..")
			if err != nil {
				t.Fatal(err)
			}
			mod := "module consumer\n\ngo 1.26.0\n\nrequire github.com/DiLRandI/confgen v0.0.0\nreplace github.com/DiLRandI/confgen => " + root + "\n"
			testCode := `package consumer
import "testing"
func TestConfig(t *testing.T) {
 c,e:=Load();if e!=nil{t.Fatal(e)}
 if c.Server.Port!=8080||c.Server.Timeout!="30s"||c.Max!=9223372036854775807||len(c.Backends)!=2||c.Backends[1].Port!=2{t.Fatal("defaults lost")}
}`
			for path, b := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "config_test.go": []byte(testCode)} {
				if err := os.WriteFile(filepath.Join(dir, path), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, arch := range []string{"amd64", "386"} {
				args := []string{"test", "-mod=mod", "./..."}
				if arch == "amd64" {
					args = append(args, "-race")
				}
				cmd := exec.Command("go", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOWORK=off", "GOARCH="+arch)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("%s: %v\n%s", arch, err, out)
				}
			}
		})
	}
}

func FuzzFromConfig(f *testing.F) {
	f.Add([]byte("port: 8080"))
	f.Add([]byte(`{"port":8080}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		for _, name := range []string{"fuzz.yaml", "fuzz.json"} {
			_, _ = infer.FromConfig(name, b, infer.Options{Package: "app"})
		}
	})
}
func BenchmarkFromConfig(b *testing.B) {
	input := []byte("server: {host: localhost, port: 8080, timeout: 30s}\nlogging: {level: info}")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := infer.FromConfig("config.yaml", input, infer.Options{Package: "app"}); err != nil {
			b.Fatal(err)
		}
	}
}
