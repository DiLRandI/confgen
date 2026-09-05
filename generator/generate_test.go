package generator_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/schema"
)

func exampleModel(t testing.TB) *schema.Model {
	t.Helper()
	b, e := os.ReadFile("../examples/appconfig/config.schema.yaml")
	if e != nil {
		t.Fatal(e)
	}
	m, e := schema.Compile("example", b)
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func TestGoldenAndDeterminism(t *testing.T) {
	m := exampleModel(t)
	a, e := generator.Generate(m, generator.Options{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := generator.Generate(m, generator.Options{})
	if e != nil || !bytes.Equal(a, b) {
		t.Fatal("nondeterministic generation")
	}
	y, e := generator.ExampleYAML(m)
	if e != nil {
		t.Fatal(e)
	}
	env, e := generator.ExampleEnv(m)
	if e != nil {
		t.Fatal(e)
	}
	for file, want := range map[string][]byte{"appconfig/config_gen.go": a, "config.example.yaml": y, "generated.env.example": env} {
		got, e := os.ReadFile("../examples/" + file)
		if e != nil || !bytes.Equal(got, want) {
			t.Fatalf("golden mismatch %s; run go -C examples generate ./...", file)
		}
	}
}

func TestFieldDocumentation(t *testing.T) {
	m, err := schema.Compile("test", []byte("version: 1\npackage: app\nfields:\n  host: {type: string, description: Address the HTTP server binds to.}\n  debug: {type: bool}\n  port: {type: int, description: Port selects the listener.}\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := generator.Generate(m, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"// Host is the address the HTTP server binds to.", "// Port selects the listener."} {
		if !bytes.Contains(b, []byte(want)) {
			t.Fatalf("missing %q", want)
		}
	}
	if bytes.Contains(b, []byte("// Debug")) {
		t.Fatal("undocumented field acquired filler documentation")
	}
}

func TestCompileGeneratedModule(t *testing.T) {
	b, e := os.ReadFile("testdata/all.schema.yaml")
	if e != nil {
		t.Fatal(e)
	}
	m, e := schema.Compile("all.schema.yaml", b)
	if e != nil {
		t.Fatal(e)
	}
	code, e := generator.Generate(m, generator.Options{})
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	root, e := filepath.Abs("..")
	if e != nil {
		t.Fatal(e)
	}
	mod := "module generatedtest\n\ngo 1.26.0\n\nrequire github.com/DiLRandI/confgen v0.0.0\nreplace github.com/DiLRandI/confgen => " + strconvQuote(root) + "\n"
	consumer, e := os.ReadFile("testdata/consumer_test.go.txt")
	if e != nil {
		t.Fatal(e)
	}
	for name, data := range map[string][]byte{"go.mod": []byte(mod), "config_gen.go": code, "consumer_test.go": consumer} {
		if e := os.WriteFile(filepath.Join(dir, name), data, 0o600); e != nil {
			t.Fatal(e)
		}
	}
	cmd := exec.Command("go", "test", "-mod=mod", "-race", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("generated consumer failed: %v\n%s", e, out)
	}
}

func strconvQuote(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }

func TestWriteFiles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.go")
	if e := os.WriteFile(a, []byte("old"), 0o600); e != nil {
		t.Fatal(e)
	}
	bad := filepath.Join(dir, "missing", "b.go")
	if e := generator.WriteFiles(map[string][]byte{a: []byte("new"), bad: []byte("bad")}); e == nil {
		t.Fatal("expected failure")
	}
	b, _ := os.ReadFile(a)
	if string(b) != "old" {
		t.Fatal("old target corrupted")
	}
	if e := generator.WriteFiles(map[string][]byte{a: []byte("new")}); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(a)
	info, _ := os.Stat(a)
	if string(b) != "new" || info.Mode().Perm() != 0o600 {
		t.Fatal("replacement or mode wrong")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary files leaked")
	}
}

func TestExamplesRedactNestedSecrets(t *testing.T) {
	m, e := schema.Compile("secret", []byte("version: 1\npackage: app\nfields:\n  servers:\n    type: list\n    default: [{password: sentinel-secret}]\n    items:\n      type: object\n      fields:\n        password: {type: string, secret: true}\n"))
	if e != nil {
		t.Fatal(e)
	}
	for _, fn := range []func(*schema.Model) ([]byte, error){generator.ExampleYAML, generator.ExampleEnv} {
		b, e := fn(m)
		if e != nil || bytes.Contains(b, []byte("sentinel-secret")) {
			t.Fatalf("unsafe template %q %v", b, e)
		}
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
