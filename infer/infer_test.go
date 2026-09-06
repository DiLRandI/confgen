package infer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestInference(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, input string }{
		{"config.yaml", "server: {port: 8080, host: localhost, timeout: 30s}\ndebug: false\nhuge: 18446744073709551615\nratio: 0.75\nbackends: [{name: a, port: 1}, {port: 2, name: b}]\norigins: [a, b]\nempty: {}\n"},
		{"config.json", `{"server":{"port":8080,"host":"localhost","timeout":"30s"},"debug":false,"huge":18446744073709551615,"ratio":0.75,"backends":[{"name":"a","port":1},{"port":2,"name":"b"}],"origins":["a","b"],"empty":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := infer.FromConfig(tc.name, []byte(tc.input), infer.Options{Package: "appconfig", CopyDefaults: true})
			require.NoError(t, err)
			fields := m.Descriptor.Fields
			require.Equal(t, "server", fields[0].Name, "unexpected fields: %+v", fields)
			require.Equal(t, "port", fields[0].Children[0].Name, "unexpected fields: %+v", fields)
			require.Equal(t, config.KindInt64, fields[0].Children[0].Kind, "unexpected fields: %+v", fields)
			require.Equal(t, config.KindString, fields[0].Children[2].Kind, "unexpected fields: %+v", fields)
			require.Equal(t, config.KindUint64, fields[2].Kind, "unexpected fields: %+v", fields)
			require.Equal(t, config.KindFloat64, fields[3].Kind, "unexpected fields: %+v", fields)
			require.True(t, fields[1].HasDefault, "incorrect default policy")
			require.Equal(t, false, fields[1].Default, "incorrect default policy")
			require.True(t, fields[4].HasDefault, "incorrect default policy")
			require.False(t, fields[4].Item.Children[0].HasDefault, "incorrect default policy")
			first, err := schema.Render(m)
			require.NoError(t, err)
			second, err := schema.Render(m)
			require.NoError(t, err, "unstable schema")
			require.Equal(t, first, second, "unstable schema")
			compiled, err := schema.Compile("bootstrap.yaml", first)
			require.NoError(t, err)
			{
				_, err = generator.Generate(compiled, generator.Options{})
				require.NoError(t, err)
			}
			require.NotContains(t, string(first), "required:", "invented metadata")
			require.NotContains(t, string(first), "secret:", "invented metadata")
			require.NotContains(t, string(first), "description:", "invented metadata")
		})
	}
}

func TestInferenceDoesNotCopyDefaultsByDefault(t *testing.T) {
	t.Parallel()
	m, err := infer.FromConfig("config.yaml", []byte("server: {port: 8080, host: localhost}\norigins: [a, b]\ndebug: false\n"), infer.Options{Package: "appconfig"})
	require.NoError(t, err)
	for _, f := range m.Descriptor.Fields {
		require.False(t, f.HasDefault, "field %q unexpectedly has a default", f.Path)
		for _, child := range f.Children {
			require.False(t, child.HasDefault, "nested field %q unexpectedly has a default", child.Path)
		}
		require.False(t, f.Item != nil && f.Item.HasDefault, "list field %q unexpectedly has an item default", f.Path)
	}
}

func TestInferenceCopiesDefaultsWhenOptedIn(t *testing.T) {
	t.Parallel()
	m, err := infer.FromConfig("config.yaml", []byte("server: {port: 8080, host: localhost}\norigins: [a, b]\ndebug: false\n"), infer.Options{Package: "appconfig", CopyDefaults: true})
	require.NoError(t, err)
	fields := m.Descriptor.Fields
	require.False(t, fields[0].HasDefault, "unexpected opt-in defaults: %+v", fields)
	require.True(t, fields[0].Children[0].HasDefault, "unexpected opt-in defaults: %+v", fields)
	require.True(t, fields[1].HasDefault, "unexpected opt-in defaults: %+v", fields)
	require.False(t, fields[1].Item.HasDefault, "unexpected opt-in defaults: %+v", fields)
}

func TestInferenceErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, input, want string }{
		{"a.yaml", "password: null", "password"},
		{"a.json", `{"password":null}`, "password"},
		{"a.yml", "values: []", "list is empty"},
		{"a.json", `{"values":[]}`, "list is empty"},
		{"a.yaml", "values: [1, hello]", "int64 and string"},
		{"a.json", `{"values":[1,"hello"]}`, "int64 and string"},
		{"a.yaml", "values: [{a: 1}, {a: hello}]", "int64 and string"},
		{"a.yaml", "values: [a, null]", "values[2]"},
		{"a.yaml", "values: [[1]]", "nested lists"},
		{"a.yaml", "x: 1\nx: 2", "duplicate"},
		{"a.json", `{"x":1,"x":2}`, "duplicate"},
		{"a.yaml", "x: &x [1]\ny: *x", "syntax"},
		{"a.yaml", "x: {<<: {a: 1}}", "syntax"},
		{"a.yaml", "x: .nan", "finite"},
		{"a.json", `{"x":1e999}`, "finite"},
		{"a.yaml", "x: 18446744073709551616", "64-bit"},
		{"a.json", `{"x":18446744073709551616}`, "64-bit"},
		{"a.yaml", "x: -9223372036854775809", "64-bit"},
		{"a.yaml", "[1, 2]", "root"},
		{"a.yaml", "x: [", "syntax"},
		{"a.toml", "x = 1", "extension"},
	} {
		t.Run(tc.name+tc.want, func(t *testing.T) {
			_, err := infer.FromConfig(tc.name, []byte(tc.input), infer.Options{Package: "app"})
			require.Error(t, err, "want %q: %v", tc.want, err)
			require.Contains(t, err.Error(), tc.want, "want %q: %v", tc.want, err)
		})
	}
}

func TestRoundTripConsumer(t *testing.T) {
	for _, name := range []string{"sample.yaml", "sample.json"} {
		t.Run(name, func(t *testing.T) {
			input := `{"server":{"port":8080,"timeout":"30s"},"max":9223372036854775807,"backends":[{"name":"a"},{"port":2}]}`
			m, err := infer.FromConfig(name, []byte(input), infer.Options{Package: "consumer", CopyDefaults: true})
			require.NoError(t, err)
			yamlBytes, err := schema.Render(m)
			require.NoError(t, err)
			m, err = schema.Compile("schema.yaml", yamlBytes)
			require.NoError(t, err)
			code, err := generator.Generate(m, generator.Options{})
			require.NoError(t, err)
			dir := t.TempDir()
			root, err := filepath.Abs("..")
			require.NoError(t, err)
			mod := "module consumer\n\ngo 1.26.0\n\nrequire github.com/DiLRandI/confgen v0.0.0\nrequire github.com/stretchr/testify v1.12.1\nreplace github.com/DiLRandI/confgen => " + root + "\n"
			testCode := `package consumer

import "github.com/stretchr/testify/require"
import "testing"

func TestConfig(t *testing.T) {
	t.Parallel()
	c, e := Load()
	require.NoError(t, e)
	require.Equal(t, int64(8080), c.Server.Port, "defaults lost")
	require.Equal(t, "30s", c.Server.Timeout, "defaults lost")
	require.Equal(t, int64(9223372036854775807), c.Max, "defaults lost")
	require.Equal(t, 2, len(c.Backends), "defaults lost")
	require.Equal(t, int64(0), c.Backends[0].Port, "defaults lost")
	require.Equal(t, int64(2), c.Backends[1].Port, "defaults lost")
}
`
			for path, b := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "config_test.go": []byte(testCode)} {
				require.NoError(t, os.WriteFile(filepath.Join(dir, path), b, 0o600))
			}
			for _, arch := range []string{"amd64", "386"} {
				args := []string{"test", "-mod=mod", "./..."}
				if arch == "amd64" {
					args = append(args, "-race")
				}
				cmd := exec.Command("go", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOWORK=off", "GOARCH="+arch)
				{
					out, err := cmd.CombinedOutput()
					require.NoError(t, err, "%s: %v\n%s", arch, err, out)
				}
			}
		})
	}
}

func FuzzFromConfig(f *testing.F) {
	f.Add([]byte("port: 8080"))
	f.Add([]byte(`{"port":8080}`))
	f.Add([]byte("fields: {port: {type: int64}}"))
	f.Fuzz(func(t *testing.T, b []byte) {
		for _, name := range []string{"fuzz.yaml", "fuzz.json"} {
			_, _ = infer.FromConfig(name, b, infer.Options{Package: "app"})
		}
		if rules, err := infer.ParseOverrides("fuzz.overrides.yaml", b); err == nil {
			_, _ = infer.FromConfig("config.yaml", []byte("port: null"), infer.Options{Overrides: rules, CopyDefaults: true})
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
