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

func runInit(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("confgen init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Bootstrap an editable schema and Go code from existing configuration.\n\nUsage: confgen init --from config.yaml --package appconfig\nExisting destinations are never overwritten.\n\nOptions:")
		fs.PrintDefaults()
	}
	from := fs.String("from", "", "existing .yaml, .yml, or .json config")
	copyDefaults := fs.Bool("copy-defaults", false, "copy inferred input values into schema defaults")
	pkg := fs.String("package", "appconfig", "generated Go package")
	schemaPath := fs.String("schema", "", "new editable schema path (default: <package>/config.schema.yaml)")
	out := fs.String("out", "", "new generated Go path (default: <package>/config_gen.go)")
	prefix := fs.String("env-prefix", "", "environment variable prefix")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if !explicit["schema"] {
		*schemaPath = filepath.Join(*pkg, "config.schema.yaml")
	}
	if !explicit["out"] {
		*out = filepath.Join(*pkg, "config_gen.go")
	}
	fail := func(err error) int { fmt.Fprintln(stderr, "confgen:", err); return 1 }
	if *from == "" || *schemaPath == "" || *out == "" || (explicit["package"] && *pkg == "") || fs.NArg() != 0 {
		return fail(fmt.Errorf("-from, package, and output paths must be non-empty"))
	}
	data, err := os.ReadFile(*from)
	if err != nil {
		return fail(err)
	}
	m, err := infer.FromConfig(*from, data, infer.Options{Package: *pkg, EnvPrefix: *prefix, CopyDefaults: *copyDefaults})
	if err != nil {
		return fail(err)
	}
	schemaBytes, err := schema.Render(m)
	if err != nil {
		return fail(err)
	}
	goBytes, err := generator.Generate(m, generator.Options{})
	if err != nil {
		return fail(err)
	}
	input, err := filepath.Abs(*from)
	if err != nil {
		return fail(err)
	}
	input, err = filepath.EvalSymlinks(input)
	if err != nil {
		return fail(err)
	}
	outputs := map[string][]byte{}
	for _, target := range []struct {
		path string
		data []byte
	}{{*schemaPath, schemaBytes}, {*out, goBytes}} {
		abs, e := filepath.Abs(target.path)
		if e != nil {
			return fail(e)
		}
		if abs == input {
			return fail(fmt.Errorf("output cannot overwrite input"))
		}
		if _, exists := outputs[abs]; exists {
			return fail(fmt.Errorf("output paths must be distinct"))
		}
		if _, e := os.Lstat(abs); e == nil {
			return fail(fmt.Errorf("destination already exists: %s", target.path))
		} else if !os.IsNotExist(e) {
			return fail(e)
		}
		outputs[abs] = target.data
	}
	for path := range outputs {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fail(err)
		}
	}
	if err := generator.CreateFiles(outputs); err != nil {
		return fail(err)
	}
	fmt.Fprintf(stderr, "Created %s and %s.\nEdit the schema, then regenerate with:\n  confgen generate -schema %q -out %q\n", *schemaPath, *out, *schemaPath, *out)
	return 0
}
