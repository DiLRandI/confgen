package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func exampleModel(t testing.TB) *schema.Model {
	t.Helper()
	b, e := os.ReadFile("../examples/appconfig/config.schema.yaml")
	require.NoError(t, e)
	m, e := schema.Compile("example", b)
	require.NoError(t, e)
	return m
}

func TestGoldenAndDeterminism(t *testing.T) {
	m := exampleModel(t)
	a, e := generator.Generate(m, generator.Options{})
	require.NoError(t, e)
	b, e := generator.Generate(m, generator.Options{})
	require.NoError(t, e, "nondeterministic generation")
	require.Equal(t, a, b, "nondeterministic generation")
	y, e := generator.ExampleYAML(m)
	require.NoError(t, e)
	env, e := generator.ExampleEnv(m)
	require.NoError(t, e)
	for file, want := range map[string][]byte{"appconfig/config_gen.go": a, "config.example.yaml": y, "generated.env.example": env} {
		got, e := os.ReadFile("../examples/" + file)
		require.NoError(t, e, "golden mismatch %s; run go -C examples generate ./...", file)
		require.Equal(t, got, want, "golden mismatch %s; run go -C examples generate ./...", file)
	}
}

func TestFieldDocumentation(t *testing.T) {
	m, err := schema.Compile("test", []byte("version: 1\npackage: app\nfields:\n  host: {type: string, description: Address the HTTP server binds to.}\n  debug: {type: bool}\n  port: {type: int, description: Port selects the listener.}\n"))
	require.NoError(t, err)
	b, err := generator.Generate(m, generator.Options{})
	require.NoError(t, err)
	for _, want := range []string{"// Host is the address the HTTP server binds to.", "// Port selects the listener."} {
		require.Contains(t, string(b), string([]byte(want)), "missing %q", want)
	}
	require.NotContains(t, string(b), "// Debug", "undocumented field acquired filler documentation")
}

func TestCompileGeneratedModule(t *testing.T) {
	b, e := os.ReadFile("testdata/all.schema.yaml")
	require.NoError(t, e)
	m, e := schema.Compile("all.schema.yaml", b)
	require.NoError(t, e)
	code, e := generator.Generate(m, generator.Options{})
	require.NoError(t, e)
	dir := t.TempDir()
	root, e := filepath.Abs("..")
	require.NoError(t, e)
	mod := "module generatedtest\n\ngo 1.26.0\n\nrequire github.com/DiLRandI/confgen v0.0.0\nrequire github.com/stretchr/testify v1.12.1\nreplace github.com/DiLRandI/confgen => " + strconvQuote(root) + "\n"
	consumer, e := os.ReadFile("testdata/consumer_test.go.txt")
	require.NoError(t, e)
	for name, data := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "consumer_test.go": consumer} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
	}
	cmd := exec.Command("go", "test", "-mod=mod", "-race", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	{
		out, e := cmd.CombinedOutput()
		require.NoError(t, e, "generated consumer failed: %v\n%s", e, out)
	}
}

func strconvQuote(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }

func TestWriteFiles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(a, []byte("old"), 0o600))
	bad := filepath.Join(dir, "missing", "b.go")
	require.Error(t, generator.WriteFiles(map[string][]byte{a: []byte("new"), bad: []byte("bad")}), "expected failure")
	b, _ := os.ReadFile(a)
	require.Equal(t, "old", string(b), "old target corrupted")
	require.NoError(t, generator.WriteFiles(map[string][]byte{a: []byte("new")}))
	b, _ = os.ReadFile(a)
	info, _ := os.Stat(a)
	require.Equal(t, "new", string(b), "replacement or mode wrong")
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "replacement or mode wrong")
	entries, _ := os.ReadDir(dir)
	require.Equal(t, 1, len(entries), "temporary files leaked")
}

func TestExamplesRedactNestedSecrets(t *testing.T) {
	m, e := schema.Compile("secret", []byte("version: 1\npackage: app\nfields:\n  servers:\n    type: list\n    default: [{password: sentinel-secret}]\n    items:\n      type: object\n      fields:\n        password: {type: string, secret: true}\n"))
	require.NoError(t, e)
	for _, fn := range []func(*schema.Model) ([]byte, error){generator.ExampleYAML, generator.ExampleEnv} {
		b, e := fn(m)
		require.NoError(t, e, "unsafe template %q %v", b, e)
		require.NotContains(t, string(b), "sentinel-secret", "unsafe template %q %v", b, e)
	}
}

func BenchmarkGenerate(b *testing.B) {
	m := exampleModel(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, e := generator.Generate(m, generator.Options{}); e != nil {
			b.Fatal(e)
		}
	}
}
