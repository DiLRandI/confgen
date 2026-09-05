package schema_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"go-config/schema"
)

func TestParseExample(t *testing.T) {
	data, err := os.ReadFile("../examples/config.schema.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.Parse("config.schema.yaml", data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 1 || s.Package != "appconfig" || s.Name != "Config" || s.EnvPrefix != "SHOP" || s.UnknownFields != "error" {
		t.Fatalf("unexpected root: %+v", s)
	}
	if len(s.Fields) != 7 || s.Fields[0].Name != "server" || s.Fields[6].Name != "labels" {
		t.Fatalf("field order lost: %+v", s.Fields)
	}
	port := s.Fields[0].Fields[1]
	if port.Path != "server.port" || port.Default.Value != "8080" || port.Default.Tag != "!!int" {
		t.Fatalf("unexpected port: %+v", port)
	}
	if port.Location.Line != 16 || port.Location.Column != 7 || port.Location.File != "config.schema.yaml" {
		t.Fatalf("location lost: %+v", port.Location)
	}
	if s.Fields[5].Items.Type != "string" || s.Fields[6].Values.Type != "string" {
		t.Fatal("collection descriptors lost")
	}
}

func TestParsePresence(t *testing.T) {
	s, err := schema.Parse("test", []byte("version: 1\npackage: test\nfields:\n  debug:\n    type: bool\n    required: false\n    default: false\n    env: false\n  empty:\n    type: string\n    default: null\n"))
	if err != nil {
		t.Fatal(err)
	}
	f := s.Fields[0]
	if !f.Has("required") || f.Required || !f.Has("default") || f.Default.Tag != "!!bool" || f.Env.Tag != "!!bool" || f.Has("secret") {
		t.Fatalf("presence lost: %+v", f)
	}
	if s.Fields[1].Default == nil || s.Fields[1].Default.Tag != "!!null" {
		t.Fatal("explicit null lost")
	}
	if s.Name != "Config" || s.UnknownFields != "error" {
		t.Fatal("root defaults missing")
	}
}

func TestParseErrors(t *testing.T) {
	prefix := "version: 1\npackage: test\nfields:\n"
	cases := []struct{ name, input, message string }{
		{"empty", "", "one valid YAML"},
		{"syntax", "fields: [", "one valid YAML"},
		{"two documents", prefix + "  a: {type: int}\n---\n{}", "exactly one"},
		{"empty second document", prefix + "  a: {type: int}\n---\n", "exactly one"},
		{"sequence root", "[a, b]", "mapping"},
		{"missing version", "package: test\nfields: {}", "missing root property version"},
		{"missing package", "version: 1\nfields: {}", "missing root property package"},
		{"missing fields", "version: 1\npackage: test", "missing root property fields"},
		{"unknown root", prefix + "  a: {type: int}\ntypo: true", "unknown root property"},
		{"duplicate root", prefix + "  a: {type: int}\nversion: 1", "duplicate mapping key"},
		{"duplicate field", prefix + "  a: {type: int}\n  a: {type: bool}", "duplicate mapping key"},
		{"duplicate property", prefix + "  a: {type: int, type: bool}", "duplicate mapping key"},
		{"duplicate default", prefix + "  a: {type: map, values: {type: string}, default: {x: a, x: b}}", "duplicate mapping key"},
		{"unknown field", prefix + "  a: {type: int, typo: 1}", "unknown field property"},
		{"unknown item", prefix + "  a: {type: list, items: {type: int, typo: 1}}", "unknown field property"},
		{"unknown map value", prefix + "  a: {type: map, values: {type: int, typo: 1}}", "unknown field property"},
		{"missing type", prefix + "  a: {default: 1}", "missing field property type"},
		{"null field", prefix + "  a: null", "mapping"},
		{"numeric key", prefix + "  1: {type: int}", "keys must be strings"},
		{"alias", prefix + "  a: &a {type: int}\n  b: *a", "aliases"},
		{"merge key", prefix + "  a: {<<: {type: int}}", "merge keys"},
		{"quoted version", "version: '1'\npackage: test\nfields: {}", "version must be an integer"},
		{"non string package", "version: 1\npackage: 123\nfields: {}", "package must be a string"},
		{"quoted bool", prefix + "  a: {type: bool, required: 'true'}", "required must be a boolean"},
		{"env true", prefix + "  a: {type: bool, env: true}", "env must be a string or false"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			s, err := schema.Parse("bad.yaml", []byte(tt.input))
			if s != nil || err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("got %v, %v; want %q", s, err, tt.message)
			}
			var d *schema.Diagnostic
			if !errors.As(err, &d) || d.Location.File != "bad.yaml" || d.Location.Line < 1 || d.Location.Column < 1 {
				t.Fatalf("missing diagnostic: %v", err)
			}
		})
	}
}

func TestParseDoesNotEchoValues(t *testing.T) {
	const secret = "unique-password-sentinel"
	_, err := schema.Parse("secret.yaml", []byte("version: 1\npackage: app\nfields:\n  password:\n    type: string\n    secret: true\n    required: "+secret))
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestParseListObject(t *testing.T) {
	s, err := schema.Parse("test", []byte("version: 1\npackage: app\nfields:\n  backends:\n    type: list\n    items:\n      type: object\n      fields:\n        url: {type: string, required: true}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f := s.Fields[0].Items.Fields[0]; f.Path != "backends[].url" || !f.Required {
		t.Fatalf("bad item: %+v", f)
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"", "version: 1\npackage: app\nfields: {}", "a: &a [*a]", "{a: 1, a: 2}"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := schema.Parse("fuzz.yaml", data)
		if err == nil && s == nil {
			t.Fatal("nil schema without error")
		}
	})
}
