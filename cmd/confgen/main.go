// Command confgen generates typed Go configuration from a YAML schema.
//
// Usage:
//
//	confgen -schema config.schema.yaml -out config_gen.go
//
// Optional -example-yaml and -example-env flags write configuration templates.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
)

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }
func run(args []string, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "init" {
		return runInit(args[1:], stderr)
	}
	if len(args) > 0 && args[0] == "generate" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("confgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Generate type-safe Go configuration.\n\nUsage:\n  confgen init --from config.yaml --package appconfig\n  confgen generate --schema config.schema.yaml --out config_gen.go\n  confgen generate --from config.json --package appconfig\n\nOptions:")
		fs.PrintDefaults()
	}
	input := fs.String("schema", "", "required YAML schema path")
	from := fs.String("from", "", "infer directly from YAML or JSON without writing a schema")
	copyDefaults := fs.Bool("copy-defaults", false, "copy inferred input values into schema defaults")
	pkg := fs.String("package", "appconfig", "Go package for inferred configuration")
	out := fs.String("out", "config_gen.go", "generated Go output")
	exYAML := fs.String("example-yaml", "", "optional YAML example output")
	exEnv := fs.String("example-env", "", "optional environment example output")
	imp := fs.String("runtime-import", "github.com/DiLRandI/confgen/config", "runtime package import path")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	fail := func(err error) int { fmt.Fprintln(stderr, "confgen:", err); return 1 }
	if (*input == "") == (*from == "") || *out == "" || fs.NArg() != 0 {
		return fail(fmt.Errorf("choose exactly one of -schema or -from and a non-empty -out"))
	}
	if *from != "" {
		*input = *from
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		return fail(err)
	}
	var m *schema.Model
	if *from != "" {
		m, err = infer.FromConfig(*input, data, infer.Options{Package: *pkg, CopyDefaults: *copyDefaults})
	} else {
		m, err = schema.Compile(*input, data)
	}
	if err != nil {
		return fail(err)
	}
	goCode, err := generator.Generate(m, generator.Options{RuntimeImport: *imp})
	if err != nil {
		return fail(err)
	}
	outputs := map[string][]byte{}
	inputAbs, err := filepath.Abs(*input)
	if err != nil {
		return fail(err)
	}
	inputAbs, err = filepath.EvalSymlinks(inputAbs)
	if err != nil {
		return fail(err)
	}
	add := func(path string, b []byte) error {
		abs, e := filepath.Abs(path)
		if e != nil {
			return e
		}
		dir, e := filepath.EvalSymlinks(filepath.Dir(abs))
		if e != nil {
			return e
		}
		abs = filepath.Join(dir, filepath.Base(abs))
		if abs == inputAbs {
			return fmt.Errorf("output cannot overwrite input")
		}
		if _, ok := outputs[abs]; ok {
			return fmt.Errorf("output paths must be distinct")
		}
		outputs[abs] = b
		return nil
	}
	if err = add(*out, goCode); err != nil {
		return fail(err)
	}
	if *exYAML != "" {
		b, e := generator.ExampleYAML(m)
		if e != nil {
			return fail(e)
		}
		if e = add(*exYAML, b); e != nil {
			return fail(e)
		}
	}
	if *exEnv != "" {
		b, e := generator.ExampleEnv(m)
		if e != nil {
			return fail(e)
		}
		if e = add(*exEnv, b); e != nil {
			return fail(e)
		}
	}
	if err = generator.WriteFiles(outputs); err != nil {
		return fail(err)
	}
	return 0
}
