package infer_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
)

func TestObjectListUnion(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{name: "yaml", input: "backends:\n  - name: a\n    auth: {token: first}\n    ports: [{name: http, port: 80}]\n  - port: 443\n    name: b\n    auth: {enabled: true}\n    ports: [{port: 443, protocol: https}]\n"},
		{name: "json", input: `{"backends":[{"name":"a","auth":{"token":"first"},"ports":[{"name":"http","port":80}]},{"port":443,"name":"b","auth":{"enabled":true},"ports":[{"port":443,"protocol":"https"}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := infer.FromConfig("config."+tc.name, []byte(tc.input), infer.Options{Package: "app", CopyDefaults: true})
			if err != nil {
				t.Fatal(err)
			}
			backends := m.Descriptor.Fields[0]
			if got := fieldNames(backends.Item.Children); got != "name,auth,ports,port" {
				t.Fatalf("backend field order = %q", got)
			}
			auth := backends.Item.Children[1]
			if got := fieldNames(auth.Children); got != "token,enabled" {
				t.Fatalf("auth field order = %q", got)
			}
			ports := backends.Item.Children[2]
			got := fieldNames(ports.Item.Children)
			if ports.Kind != config.KindList || got != "name,port,protocol" {
				t.Fatalf("port field order = %q", got)
			}
			assertNoDefaults(t, *backends.Item)
			first, err := schema.Render(m)
			if err != nil {
				t.Fatal(err)
			}
			second, err := schema.Render(m)
			if err != nil || !bytes.Equal(first, second) {
				t.Fatal("union schema render is not deterministic")
			}

			withoutDefaults, err := infer.FromConfig("config."+tc.name, []byte(tc.input), infer.Options{Package: "app"})
			if err != nil {
				t.Fatal(err)
			}
			assertNoDefaults(t, withoutDefaults.Descriptor.Fields[0])
		})
	}
}

func TestObjectListUnionConflictsUseCanonicalPaths(t *testing.T) {
	for _, tc := range []struct {
		name, input, kinds string
	}{
		{name: "yaml", input: "backends:\n  - port: 17\n  - port: {nested: true}\n", kinds: "int64 and object"},
		{name: "json", input: `{"backends":[{"port":17},{"port":{"nested":true}}]}`, kinds: "int64 and object"},
		{name: "yaml-reverse", input: "backends:\n  - port: {nested: true}\n  - port: 17\n", kinds: "object and int64"},
		{name: "json-reverse", input: `{"backends":[{"port":{"nested":true}},{"port":17}]}`, kinds: "object and int64"},
		{name: "yaml-scalar", input: "backends: [{port: 17}, {port: '17'}]", kinds: "int64 and string"},
		{name: "json-scalar", input: `{"backends":[{"port":17},{"port":"17"}]}`, kinds: "int64 and string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			format, _, _ := strings.Cut(tc.name, "-")
			_, err := infer.FromConfig("conflict."+format, []byte(tc.input), infer.Options{Package: "app"})
			if err == nil || !strings.Contains(err.Error(), `"backends[].port"`) || !strings.Contains(err.Error(), tc.kinds) || strings.Contains(err.Error(), "17") || strings.Contains(err.Error(), "nested") {
				t.Fatalf("unexpected conflict error: %v", err)
			}
		})
	}
}

func fieldNames(fields []config.FieldDescriptor) string {
	var names []string
	for _, f := range fields {
		names = append(names, f.Name)
	}
	return strings.Join(names, ",")
}

func assertNoDefaults(t *testing.T, f config.FieldDescriptor) {
	t.Helper()
	if f.HasDefault {
		t.Fatalf("inferred item field %q has a default", f.Path)
	}
	for _, child := range f.Children {
		assertNoDefaults(t, child)
	}
	if f.Item != nil {
		assertNoDefaults(t, *f.Item)
	}
}
