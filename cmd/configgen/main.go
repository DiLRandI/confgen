// Command configgen generates typed Go configuration from a YAML schema.
//
// Usage:
//
//	configgen -schema config.schema.yaml -out config_gen.go
//
// Optional -example-yaml and -example-env flags write configuration templates.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go-config/generator"
	"go-config/schema"
)

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }
func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("configgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	input := fs.String("schema", "", "required YAML schema path")
	out := fs.String("out", "config_gen.go", "generated Go output")
	exYAML := fs.String("example-yaml", "", "optional YAML example output")
	exEnv := fs.String("example-env", "", "optional environment example output")
	imp := fs.String("runtime-import", "go-config/config", "runtime package import path")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	fail := func(err error) int { fmt.Fprintln(stderr, "configgen:", err); return 1 }
	if *input == "" || *out == "" || fs.NArg() != 0 {
		return fail(fmt.Errorf("-schema and -out are required; positional arguments are unsupported"))
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		return fail(err)
	}
	m, err := schema.Compile(*input, data)
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
			return fmt.Errorf("output cannot overwrite schema")
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
