package infer_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
)

func TestSparseOverrides(t *testing.T) {
	rules, err := infer.ParseOverrides("overrides.yaml", []byte(`fields:
  database_url: {type: string}
  origins: {type: list, items: {type: string}}
  server.timeout: {type: duration}
  labels: {type: map, values: {type: string}}
`))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ ext, input string }{
		{"yaml", "databaseURL: null\norigins: []\nserver: {timeout: 30s}\nlabels: {some.key: value}"},
		{"json", `{"databaseURL":null,"origins":[],"server":{"timeout":"30s"},"labels":{"some.key":"value"}}`},
	} {
		for _, copyDefaults := range []bool{false, true} {
			m, err := infer.FromConfig("config."+tc.ext, []byte(tc.input), infer.Options{Package: "app", Overrides: rules, CopyDefaults: copyDefaults})
			if err != nil {
				t.Fatal(err)
			}
			f := m.Descriptor.Fields
			if f[0].Kind != config.KindString || f[0].HasDefault || f[1].Item.Kind != config.KindString || f[2].Children[0].Kind != config.KindDuration || f[3].MapValue.Kind != config.KindString {
				t.Fatal("override types lost")
			}
			if f[1].HasDefault != copyDefaults || f[2].Children[0].HasDefault != copyDefaults || f[3].HasDefault != copyDefaults {
				t.Fatal("default policy changed")
			}
			b, err := schema.Render(m)
			if err != nil {
				t.Fatal(err)
			}
			again, err := schema.Render(m)
			if err != nil || !bytes.Equal(b, again) {
				t.Fatal("nondeterministic render")
			}
			round, err := schema.Compile("round.yaml", b)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := generator.Generate(round, generator.Options{}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestOverrideErrors(t *testing.T) {
	for _, tc := range []struct{ input, rules, want string }{
		{"server: {}", "fields: {server: {type: int}}", "conflicts with object"},
		{"server: value", "fields: {server: {type: object}}", "requires an object"},
		{"port: 1", "fields: {foo: {type: string}}", `unknown field "foo"`},
		{"server: {port: 1}", "fields: {server.foo: {type: string}}", `unknown field "server.foo"`},
		{"origins: []", "fields: {origins: {type: list}}", "requires an item type"},
		{"labels: {}", "fields: {labels: {type: map}}", "requires a value type"},
		{"timeout: invalid-sensitive-marker", "fields: {timeout: {type: duration}}", "incompatible"},
		{"port: '123'", "fields: {port: {type: int}}", "incompatible"},
		{"origins: [1]", "fields: {origins: {type: list, items: {type: string}}}", "incompatible"},
		{"labels: {a: true}", "fields: {labels: {type: map, values: {type: string}}}", "incompatible"},
	} {
		rules, err := infer.ParseOverrides("overrides.yaml", []byte(tc.rules))
		if err == nil {
			_, err = infer.FromConfig("config.yaml", []byte(tc.input), infer.Options{Overrides: rules, CopyDefaults: true})
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "invalid-sensitive-marker") {
			t.Fatalf("want %s, got %v", tc.want, err)
		}
	}
}

func TestParseOverridesRejectsUnsupportedMetadata(t *testing.T) {
	for _, input := range []string{
		"[]", "fields: []", "extra: {}", "fields: {x: {type: string, secret: true}}", "fields: {x: {type: string, default: raw-secret}}", "fields: {x: {type: unknown}}", "fields: {x: {type: string, items: {type: int}}}", "fields: {x: {type: list, items: {type: object}}}", "fields: {x: {type: list, items: {type: list, items: {type: string}}}}", "fields: {x: {type: int}, x: {type: string}}", "fields: {server-port: {type: string}}", "fields: {x: {type: string, values: {type: string}}}",
	} {
		if _, err := infer.ParseOverrides("bad.yaml", []byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestOverrideNullDefaultsOmitted(t *testing.T) {
	for _, tc := range []struct {
		input string
		rule  infer.TypeOverride
	}{
		{"x: null", infer.TypeOverride{Type: config.KindString}},
		{"x: null", infer.TypeOverride{Type: config.KindList, Items: &infer.TypeOverride{Type: config.KindString}}},
		{"x: [a, null]", infer.TypeOverride{Type: config.KindList, Items: &infer.TypeOverride{Type: config.KindString}}},
		{"x: {a: null}", infer.TypeOverride{Type: config.KindMap, Values: &infer.TypeOverride{Type: config.KindString}}},
	} {
		m, err := infer.FromConfig("config.yaml", []byte(tc.input), infer.Options{CopyDefaults: true, Overrides: map[string]infer.TypeOverride{"x": tc.rule}})
		if err != nil {
			t.Fatal(err)
		}
		if m.Descriptor.Fields[0].HasDefault {
			t.Fatal("null-containing default copied")
		}
	}
}

func TestOverrideGeneratedConsumer(t *testing.T) {
	m, err := infer.FromConfig("config.yaml", []byte("url: null\norigins: []\ntimeout: 30s"), infer.Options{Package: "consumer", CopyDefaults: true, Overrides: map[string]infer.TypeOverride{
		"url": {Type: config.KindString}, "origins": {Type: config.KindList, Items: &infer.TypeOverride{Type: config.KindString}}, "timeout": {Type: config.KindDuration},
	}})
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
	mod := "module consumer\n\ngo 1.26.0\nrequire github.com/DiLRandI/confgen v0.0.0\nreplace github.com/DiLRandI/confgen => " + strconv.Quote(root) + "\n"
	consumer := `package consumer
import("testing";"time";"strings";"github.com/DiLRandI/confgen/config")
func TestTyped(t *testing.T){
 c,e:=Load();if e!=nil{t.Fatal(e)}
 if c.URL!=""||c.Origins==nil||len(c.Origins)!=0||c.Timeout!=30*time.Second{t.Fatal("default policy lost")}
 c,e=Load(config.Reader("runtime",strings.NewReader("url: local\norigins: [a]\ntimeout: 1s"),config.FormatYAML));if e!=nil||c.Timeout!=time.Second||c.URL!="local"{t.Fatal("typed load failed",e)}
 if _,e=Load(config.Reader("null",strings.NewReader("url: null"),config.FormatYAML));e==nil{t.Fatal("runtime null semantics changed")}
}
`
	for name, data := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "consumer_test.go": []byte(consumer)} {
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
