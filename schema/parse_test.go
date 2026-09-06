package schema_test

import (
	"os"
	"testing"

	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestParseExample(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../examples/appconfig/config.schema.yaml")
	require.NoError(t, err)
	s, err := schema.Parse("config.schema.yaml", data)
	require.NoError(t, err)
	require.Equal(t, 1, s.Version, "unexpected root: %+v", s)
	require.Equal(t, "appconfig", s.Package, "unexpected root: %+v", s)
	require.Equal(t, "Config", s.Name, "unexpected root: %+v", s)
	require.Equal(t, "SHOP", s.EnvPrefix, "unexpected root: %+v", s)
	require.Equal(t, "error", s.UnknownFields, "unexpected root: %+v", s)
	require.Equal(t, 7, len(s.Fields), "field order lost: %+v", s.Fields)
	require.Equal(t, "server", s.Fields[0].Name, "field order lost: %+v", s.Fields)
	require.Equal(t, "labels", s.Fields[6].Name, "field order lost: %+v", s.Fields)
	port := s.Fields[0].Fields[1]
	require.Equal(t, "server.port", port.Path, "unexpected port: %+v", port)
	require.Equal(t, "8080", port.Default.Value, "unexpected port: %+v", port)
	require.Equal(t, "!!int", port.Default.Tag, "unexpected port: %+v", port)
	require.Equal(t, 16, port.Location.Line, "location lost: %+v", port.Location)
	require.Equal(t, 7, port.Location.Column, "location lost: %+v", port.Location)
	require.Equal(t, "config.schema.yaml", port.Location.File, "location lost: %+v", port.Location)
	require.Equal(t, "string", s.Fields[5].Items.Type, "collection descriptors lost")
	require.Equal(t, "string", s.Fields[6].Values.Type, "collection descriptors lost")
}

func TestParsePresence(t *testing.T) {
	t.Parallel()
	s, err := schema.Parse("test", []byte("version: 1\npackage: test\nfields:\n  debug:\n    type: bool\n    required: false\n    default: false\n    env: false\n  empty:\n    type: string\n    default: null\n"))
	require.NoError(t, err)
	f := s.Fields[0]
	require.True(t, f.Has("required"), "presence lost: %+v", f)
	require.False(t, f.Required, "presence lost: %+v", f)
	require.True(t, f.Has("default"), "presence lost: %+v", f)
	require.Equal(t, "!!bool", f.Default.Tag, "presence lost: %+v", f)
	require.Equal(t, "!!bool", f.Env.Tag, "presence lost: %+v", f)
	require.False(t, f.Has("secret"), "presence lost: %+v", f)
	require.NotNil(t, s.Fields[1].Default, "explicit null lost")
	require.Equal(t, "!!null", s.Fields[1].Default.Tag, "explicit null lost")
	require.Equal(t, "Config", s.Name, "root defaults missing")
	require.Equal(t, "error", s.UnknownFields, "root defaults missing")
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			s, err := schema.Parse("bad.yaml", []byte(tt.input))
			require.Nil(t, s, "got %v, %v; want %q", s, err, tt.message)
			require.Error(t, err, "got %v, %v; want %q", s, err, tt.message)
			require.Contains(t, err.Error(), tt.message, "got %v, %v; want %q", s, err, tt.message)
			var d *schema.Diagnostic
			require.ErrorAs(t, err, &d, "missing diagnostic: %v", err)
			require.Equal(t, "bad.yaml", d.Location.File, "missing diagnostic: %v", err)
			require.GreaterOrEqual(t, d.Location.Line, 1, "missing diagnostic: %v", err)
			require.GreaterOrEqual(t, d.Location.Column, 1, "missing diagnostic: %v", err)
		})
	}
}

func TestParseDoesNotEchoValues(t *testing.T) {
	t.Parallel()
	const secret = "unique-password-sentinel"
	_, err := schema.Parse("secret.yaml", []byte("version: 1\npackage: app\nfields:\n  password:\n    type: string\n    secret: true\n    required: "+secret))
	require.Error(t, err, "unsafe error: %v", err)
	require.NotContains(t, err.Error(), secret, "unsafe error: %v", err)
}

func TestParseListObject(t *testing.T) {
	t.Parallel()
	s, err := schema.Parse("test", []byte("version: 1\npackage: app\nfields:\n  backends:\n    type: list\n    items:\n      type: object\n      fields:\n        url: {type: string, required: true}\n"))
	require.NoError(t, err)
	{
		f := s.Fields[0].Items.Fields[0]
		require.Equal(t, "backends[].url", f.Path, "bad item: %+v", f)
		require.True(t, f.Required, "bad item: %+v", f)
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"", "version: 1\npackage: app\nfields: {}", "a: &a [*a]", "{a: 1, a: 2}"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := schema.Parse("fuzz.yaml", data)
		if err == nil {
			require.NotNil(t, s, "nil schema without error")
		}
	})
}
