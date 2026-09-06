package schema_test

import (
	"os"
	"testing"

	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestCompileExample(t *testing.T) {
	t.Parallel()
	b, e := os.ReadFile("../examples/appconfig/config.schema.yaml")
	require.NoError(t, e)
	m, e := schema.Compile("example", b)
	require.NoError(t, e)
	f := m.Descriptor.Fields[0].Children[1]
	require.Equal(t, "Port", f.GoName, "%+v", f)
	require.Equal(t, "SHOP_SERVER_PORT", f.EnvName, "%+v", f)
	require.True(t, f.HasDefault, "%+v", f)
	require.Equal(t, "DATABASE_URL", m.Descriptor.Fields[1].Children[0].EnvName, "explicit env lost")
}

func TestValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, field string
		valid       bool
	}{
		{"string", "type: string, default: hello, min_length: 1, max_length: 9, pattern: '^h', enum: [hello, hey]", true},
		{"bool", "type: bool, required: true, default: false", true},
		{"int", "type: int, default: 0", true},
		{"int64", "type: int64, default: 9223372036854775807", true},
		{"uint", "type: uint, default: 0", true},
		{"uint64", "type: uint64, default: 18446744073709551615", true},
		{"float", "type: float64, default: 1.25", true},
		{"duration", "type: duration, default: 1h30m, min: 1m, max: 2h", true},
		{"path", "type: path, default: ./data", true},
		{"list", "type: list, items: {type: int}, default: [1, 2]", true},
		{"map", "type: map, values: {type: string}, default: {x: y}", true},
		{"object", "type: object, fields: {port: {type: int}}", true},
		{"list object", "type: list, items: {type: object, fields: {id: {type: int, required: true}}}, default: [{id: 0}]", true},
		{"unknown type", "type: integer", false},
		{"no fields", "type: object", false},
		{"no items", "type: list", false},
		{"no values", "type: map", false},
		{"map object", "type: map, values: {type: object, fields: {}}", false},
		{"required object", "type: object, fields: {}, required: false", false},
		{"object default", "type: object, fields: {}, default: {}", false},
		{"bad default", "type: int, default: '12'", false},
		{"overflow", "type: int64, default: 9223372036854775808", false},
		{"negative unsigned", "type: uint64, default: -1", false},
		{"nan", "type: float64, default: .nan", false},
		{"invalid duration", "type: duration, default: 2d", false},
		{"null", "type: string, default: null", false},
		{"bad bounds", "type: int, min: 5, max: 1", false},
		{"bad duration bound", "type: duration, min: 2d", false},
		{"null bound", "type: int, min: null", false},
		{"wrong bounds", "type: string, min: 1", false},
		{"bad length", "type: string, min_length: -1", false},
		{"length order", "type: string, min_length: 3, max_length: 2", false},
		{"pattern", "type: string, pattern: '['", false},
		{"enum duplicate", "type: string, enum: [a, a]", false},
		{"enum type", "type: int, enum: [a]", false},
		{"enum default", "type: string, enum: [a], default: b", false},
		{"list default", "type: list, items: {type: int}, default: [a]", false},
		{"list missing required", "type: list, items: {type: object, fields: {id: {type: int, required: true}}}, default: [{}]", false},
		{"list unknown", "type: list, items: {type: object, fields: {}}, default: [{typo: 0}]", false},
		{"bad go name", "type: string, go_name: private", false},
		{"env disabled", "type: string, env: false", true},
		{"bad env", "type: string, env: ''", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, e := schema.Compile("test", []byte("version: 1\npackage: app\nfields:\n  field: {"+c.field+"}\n"))
			if c.valid {
				require.NoError(t, e)
			} else {
				require.Error(t, e)
			}
		})
	}
}

func TestRootAndCollisions(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"version: 2\npackage: app\nfields: {}",
		"version: 1\npackage: for\nfields: {}",
		"version: 1\npackage: app\nname: Load\nfields: {}",
		"version: 1\npackage: app\nname: config\nfields: {}",
		"version: 1\npackage: app\nunknown_fields: typo\nfields: {}",
		"version: 1\npackage: app\nfields: {Bad: {type: string}}",
		"version: 1\npackage: app\nfields: {api_url: {type: string}, other: {type: string, go_name: APIURL}}",
		"version: 1\npackage: app\nfields: {a_b: {type: string}, a: {type: object, fields: {b: {type: string}}}}",
		"version: 1\npackage: app\nname: AConfig\nfields: {a: {type: object, fields: {}}}",
	} {
		{
			_, e := schema.Compile("test", []byte(input))
			require.Error(t, e, "accepted %s", input)
		}
	}
}

func TestGoName(t *testing.T) {
	t.Parallel()
	for k, w := range map[string]string{"api_url": "APIURL", "http_port": "HTTPPort", "database_id": "DatabaseID", "tls_enabled": "TLSEnabled", "server_port": "ServerPort"} {
		{
			g := schema.GoName(k)
			require.Equal(t, w, g, "%s: %s", k, g)
		}
	}
}

func TestSecretDefaultDiagnostic(t *testing.T) {
	t.Parallel()
	_, e := schema.Compile("secret", []byte("version: 1\npackage: app\nfields: {password: {type: duration, secret: true, default: sentinel-secret}}"))
	require.Error(t, e, "%v", e)
	require.NotContains(t, e.Error(), "sentinel-secret", "%v", e)
}

func FuzzCompile(f *testing.F) {
	f.Add([]byte("version: 1\npackage: app\nfields: {}"))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = schema.Compile("fuzz", b) })
}
