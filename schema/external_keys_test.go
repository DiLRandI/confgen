package schema_test

import (
	"strconv"
	"testing"

	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestExternalKeys(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"snake_case", "camelCase", "PascalCase", "kebab-case", "some.key", "clé"} {
		m, err := schema.Compile("key.yaml", []byte("version: 1\npackage: app\nenv_prefix: APP\nfields:\n  canonical: {type: string, key: "+strconv.Quote(key)+"}\n"))
		require.NoError(t, err)
		f := m.Descriptor.Fields[0]
		require.Equal(t, key, f.ExternalKey(), "names conflated")
		require.Equal(t, "canonical", f.Name, "names conflated")
		require.Equal(t, "canonical", f.Path, "names conflated")
		require.Equal(t, "Canonical", f.GoName, "names conflated")
		require.Equal(t, "APP_CANONICAL", f.EnvName, "names conflated")
	}
}

func TestInvalidExternalKeys(t *testing.T) {
	t.Parallel()
	for _, fields := range []string{
		"a: {type: string, key: ''}",
		"a: {type: string, key: 123}",
		"a: {type: string, key: '-'}",
		"a: {type: string, key: 'a,b'}",
		"a: {type: string, key: 'a`b'}",
		"a: {type: string, key: 'a\"b'}",
		"a: {type: string, key: 'a\\b'}",
		"a: {type: string, key: \"a\\nb\"}",
		"a: {type: string, key: other}\n  b: {type: string, key: other}",
		"a: {type: string, key: b}\n  b: {type: string}",
		"a: {type: string}\n  a: {type: bool}",
		"a: {type: string, go_name: Same}\n  b: {type: string, go_name: Same}",
		"a: {type: list, items: {type: string, key: invalid}}",
		"a: {type: map, values: {type: string, key: invalid}}",
		"a: {type: object, fields: {b: {type: string, key: x}, c: {type: string, key: x}}}",
	} {
		{
			_, err := schema.Compile("bad.yaml", []byte("version: 1\npackage: app\nfields:\n  "+fields+"\n"))
			require.Error(t, err, "accepted %s", fields)
		}
	}
	m, err := schema.Compile("old.yaml", []byte("version: 1\npackage: app\nfields:\n  unchanged: {type: string}\n"))
	require.NoError(t, err)
	b, err := schema.Render(m)
	require.NoError(t, err, "legacy key changed: %v\n%s", err, b)
	require.NotContains(t, string(b), "key:", "legacy key changed: %v\n%s", err, b)
}
