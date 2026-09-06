package schema_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/schema"
)

func TestExternalKeys(t *testing.T) {
	for _, key := range []string{"snake_case", "camelCase", "PascalCase", "kebab-case", "some.key", "clé"} {
		m, err := schema.Compile("key.yaml", []byte("version: 1\npackage: app\nenv_prefix: APP\nfields:\n  canonical: {type: string, key: "+strconv.Quote(key)+"}\n"))
		if err != nil {
			t.Fatal(err)
		}
		f := m.Descriptor.Fields[0]
		if f.ExternalKey() != key || f.Name != "canonical" || f.Path != "canonical" || f.GoName != "Canonical" || f.EnvName != "APP_CANONICAL" {
			t.Fatal("names conflated")
		}
	}
}

func TestInvalidExternalKeys(t *testing.T) {
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
		if _, err := schema.Compile("bad.yaml", []byte("version: 1\npackage: app\nfields:\n  "+fields+"\n")); err == nil {
			t.Fatalf("accepted %s", fields)
		}
	}
	m, err := schema.Compile("old.yaml", []byte("version: 1\npackage: app\nfields:\n  unchanged: {type: string}\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := schema.Render(m)
	if err != nil || strings.Contains(string(b), "key:") {
		t.Fatalf("legacy key changed: %v\n%s", err, b)
	}
}
