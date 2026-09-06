package infer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestSparseOverrides(t *testing.T) {
	t.Parallel()
	rules, err := infer.ParseOverrides("overrides.yaml", []byte(`fields:
  database_url: {type: string}
  origins: {type: list, items: {type: string}}
  server.timeout: {type: duration}
  labels: {type: map, values: {type: string}}
`))
	require.NoError(t, err)
	for _, tc := range []struct{ ext, input string }{
		{"yaml", "databaseURL: null\norigins: []\nserver: {timeout: 30s}\nlabels: {some.key: value}"},
		{"json", `{"databaseURL":null,"origins":[],"server":{"timeout":"30s"},"labels":{"some.key":"value"}}`},
	} {
		for _, copyDefaults := range []bool{false, true} {
			m, err := infer.FromConfig("config."+tc.ext, []byte(tc.input), infer.Options{Package: "app", Overrides: rules, CopyDefaults: copyDefaults})
			require.NoError(t, err)
			f := m.Descriptor.Fields
			require.Equal(t, config.KindString, f[0].Kind)
			require.False(t, f[0].HasDefault)
			require.Equal(t, config.KindString, f[1].Item.Kind)
			require.Equal(t, config.KindDuration, f[2].Children[0].Kind)
			require.Equal(t, config.KindString, f[3].MapValue.Kind)
			require.Equal(t, copyDefaults, f[1].HasDefault)
			require.Equal(t, copyDefaults, f[2].Children[0].HasDefault)
			require.Equal(t, copyDefaults, f[3].HasDefault)
			b, err := schema.Render(m)
			require.NoError(t, err)
			again, err := schema.Render(m)
			require.NoError(t, err)
			require.Equal(t, b, again)
			round, err := schema.Compile("round.yaml", b)
			require.NoError(t, err)
			_, err = generator.Generate(round, generator.Options{})
			require.NoError(t, err)
		}
	}
}

func TestOverrideErrors(t *testing.T) {
	t.Parallel()
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
		require.Error(t, err)
		require.Contains(t, err.Error(), tc.want)
		require.NotContains(t, err.Error(), "invalid-sensitive-marker")
	}
}

func TestParseOverridesRejectsUnsupportedMetadata(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"[]", "fields: []", "extra: {}", "fields: {x: {type: string, secret: true}}", "fields: {x: {type: string, default: raw-secret}}", "fields: {x: {type: unknown}}", "fields: {x: {type: string, items: {type: int}}}", "fields: {x: {type: list, items: {type: object}}}", "fields: {x: {type: list, items: {type: list, items: {type: string}}}}", "fields: {x: {type: int}, x: {type: string}}", "fields: {server-port: {type: string}}", "fields: {x: {type: string, values: {type: string}}}",
	} {
		_, err := infer.ParseOverrides("bad.yaml", []byte(input))
		require.Error(t, err)
	}
}

func TestOverrideNullDefaultsOmitted(t *testing.T) {
	t.Parallel()
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
		require.NoError(t, err)
		require.False(t, m.Descriptor.Fields[0].HasDefault)
	}
}

func TestOverrideGeneratedConsumer(t *testing.T) {
	m, err := infer.FromConfig("config.yaml", []byte("url: null\norigins: []\ntimeout: 30s"), infer.Options{Package: "consumer", CopyDefaults: true, Overrides: map[string]infer.TypeOverride{
		"url": {Type: config.KindString}, "origins": {Type: config.KindList, Items: &infer.TypeOverride{Type: config.KindString}}, "timeout": {Type: config.KindDuration},
	}})
	require.NoError(t, err)
	code, err := generator.Generate(m, generator.Options{})
	require.NoError(t, err)
	dir := t.TempDir()
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	mod := "module consumer\n\ngo 1.26.0\nrequire github.com/DiLRandI/confgen v0.0.0\nrequire github.com/stretchr/testify v1.12.1\nreplace github.com/DiLRandI/confgen => " + strconv.Quote(root) + "\n"
	consumer := `package consumer
import("testing";"time";"strings";"github.com/DiLRandI/confgen/config";"github.com/stretchr/testify/require")
func TestTyped(t *testing.T){
 c,e:=Load()
 require.NoError(t,e)
 require.Empty(t,c.URL)
 require.NotNil(t,c.Origins)
 require.Empty(t,c.Origins)
 require.Equal(t,30*time.Second,c.Timeout)
 c,e=Load(config.Reader("runtime",strings.NewReader("url: local\norigins: [a]\ntimeout: 1s"),config.FormatYAML))
 require.NoError(t,e)
 require.Equal(t,time.Second,c.Timeout)
 require.Equal(t,"local",c.URL)
 _,e=Load(config.Reader("null",strings.NewReader("url: null"),config.FormatYAML))
 require.Error(t,e)
}
`
	for name, data := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "consumer_test.go": []byte(consumer)} {
		err := os.WriteFile(filepath.Join(dir, name), data, 0o600)
		require.NoError(t, err)
	}
	cmd := exec.Command("go", "test", "-mod=mod", "-race", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "consumer output: %s", out)
}
