package infer_test

import (
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestObjectListUnion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		input string
	}{
		{name: "yaml", input: "backends:\n  - name: a\n    auth: {token: first}\n    ports: [{name: http, port: 80}]\n  - port: 443\n    name: b\n    auth: {enabled: true}\n    ports: [{port: 443, protocol: https}]\n"},
		{name: "json", input: `{"backends":[{"name":"a","auth":{"token":"first"},"ports":[{"name":"http","port":80}]},{"port":443,"name":"b","auth":{"enabled":true},"ports":[{"port":443,"protocol":"https"}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m, err := infer.FromConfig("config."+tc.name, []byte(tc.input), infer.Options{Package: "app", CopyDefaults: true})
			require.NoError(t, err)
			backends := m.Descriptor.Fields[0]
			{
				got := fieldNames(backends.Item.Children)
				require.Equal(t, "name,auth,ports,port", got, "backend field order = %q", got)
			}
			auth := backends.Item.Children[1]
			{
				got := fieldNames(auth.Children)
				require.Equal(t, "token,enabled", got, "auth field order = %q", got)
			}
			ports := backends.Item.Children[2]
			got := fieldNames(ports.Item.Children)
			require.Equal(t, config.KindList, ports.Kind, "port field order = %q", got)
			require.Equal(t, "name,port,protocol", got, "port field order = %q", got)
			assertNoDefaults(t, *backends.Item)
			first, err := schema.Render(m)
			require.NoError(t, err)
			second, err := schema.Render(m)
			require.NoError(t, err, "union schema render is not deterministic")
			require.Equal(t, first, second, "union schema render is not deterministic")

			withoutDefaults, err := infer.FromConfig("config."+tc.name, []byte(tc.input), infer.Options{Package: "app"})
			require.NoError(t, err)
			assertNoDefaults(t, withoutDefaults.Descriptor.Fields[0])
		})
	}
}

func TestObjectListUnionConflictsUseCanonicalPaths(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			format, _, _ := strings.Cut(tc.name, "-")
			_, err := infer.FromConfig("conflict."+format, []byte(tc.input), infer.Options{Package: "app"})
			require.Error(t, err, "unexpected conflict error: %v", err)
			require.Contains(t, err.Error(), `"backends[].port"`, "unexpected conflict error: %v", err)
			require.Contains(t, err.Error(), tc.kinds, "unexpected conflict error: %v", err)
			require.NotContains(t, err.Error(), "17", "unexpected conflict error: %v", err)
			require.NotContains(t, err.Error(), "nested", "unexpected conflict error: %v", err)
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
	require.False(t, f.HasDefault, "inferred item field %q has a default", f.Path)
	for _, child := range f.Children {
		assertNoDefaults(t, child)
	}
	if f.Item != nil {
		assertNoDefaults(t, *f.Item)
	}
}
