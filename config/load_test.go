package config_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DiLRandI/confgen/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	Server struct {
		Host    string
		Port    int
		Timeout time.Duration
	}
	Debug  bool
	Name   string
	List   []string
	Labels map[string]string
}

func descriptor() config.Descriptor {
	return config.Descriptor{Fields: []config.FieldDescriptor{
		{Name: "server", Kind: config.KindObject, GoIndex: []int{0}, Children: []config.FieldDescriptor{
			{Name: "host", Path: "server.host", Kind: config.KindString, GoIndex: []int{0}, EnvName: "APP_HOST", HasDefault: true, Default: "localhost"},
			{Name: "port", Path: "server.port", Kind: config.KindInt, GoIndex: []int{1}, EnvName: "APP_PORT", Required: true},
			{Name: "timeout", Path: "server.timeout", Kind: config.KindDuration, GoIndex: []int{2}, EnvName: "APP_TIMEOUT"},
		}},
		{Name: "debug", Path: "debug", Kind: config.KindBool, GoIndex: []int{1}, EnvName: "APP_DEBUG", Required: true},
		{Name: "name", Path: "name", Kind: config.KindString, GoIndex: []int{2}, EnvName: "EXPLICIT_NAME", Required: true},
		{Name: "list", Path: "list", Kind: config.KindList, GoIndex: []int{3}, EnvName: "APP_LIST", Item: &config.FieldDescriptor{Kind: config.KindString}},
		{Name: "labels", Path: "labels", Kind: config.KindMap, GoIndex: []int{4}, EnvName: "APP_LABELS", MapValue: &config.FieldDescriptor{Kind: config.KindString}},
	}}
}

func reader(s string) config.Source {
	return config.Reader("test", strings.NewReader(s), config.FormatYAML)
}

func environment(m map[string]string) config.Source {
	return config.Env(config.WithLookupEnv(func(k string) (string, bool) { v, ok := m[k]; return v, ok }))
}

func issue(t *testing.T, err error, kind config.IssueKind) {
	t.Helper()
	var e *config.Error
	require.ErrorAs(t, err, &e)
	require.NotEmpty(t, e.Issues)
	assert.Equal(t, kind, e.Issues[0].Kind)
}

func TestInvalidObjectShape(t *testing.T) {
	t.Parallel()
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Name: "server", Kind: config.KindObject, GoIndex: []int{0}, Children: []config.FieldDescriptor{{Name: "host", Path: "server.host", Kind: config.KindString, GoIndex: []int{0}}}}}}
	c, err := config.Load[testConfig](context.Background(), d, reader("server: scalar"))
	assert.Nil(t, c)
	issue(t, err, config.IssueType)
}

func TestEmptyObjectNull(t *testing.T) {
	t.Parallel()
	type cfg struct{ Empty struct{} }
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Name: "empty", Path: "empty", Kind: config.KindObject, GoIndex: []int{0}}}}
	c, err := config.Load[cfg](context.Background(), d, reader("empty: null"))
	assert.Nil(t, c)
	require.Error(t, err)
	_, err = config.Load[cfg](context.Background(), d, reader("empty: null"), reader("empty: {}"))
	require.NoError(t, err)
}

func TestTimestampConversionAfterMerge(t *testing.T) {
	t.Parallel()
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Name: "name", Path: "name", Kind: config.KindString, GoIndex: []int{0}, EnvName: "NAME"}}}
	low := reader("name: 2026-09-05")
	_, err := config.Load[struct{ Name string }](context.Background(), d, low)
	require.Error(t, err, "timestamp accepted as string")
	c, err := config.Load[struct{ Name string }](context.Background(), d, low, environment(map[string]string{"NAME": "overridden"}))
	require.NoError(t, err)
	assert.Equal(t, "overridden", c.Name)
}

func TestPresencePrecedenceAndCollections(t *testing.T) {
	t.Parallel()
	d := descriptor()
	low := reader("server: {host: original, port: wrong}\ndebug: true\nname: previous\nlist: [a, b]\nlabels: {a: one, b: two}")
	env := environment(map[string]string{"APP_PORT": "0", "APP_DEBUG": "false", "EXPLICIT_NAME": "", "APP_LIST": "[]", "APP_LABELS": "{}", "APP_TIMEOUT": "1h30m"})
	c, err := config.Load[testConfig](context.Background(), d, low, env)
	require.NoError(t, err)
	assert.Zero(t, c.Server.Port)
	assert.Equal(t, "original", c.Server.Host)
	assert.Equal(t, 90*time.Minute, c.Server.Timeout)
	assert.False(t, c.Debug)
	assert.Empty(t, c.Name)
	assert.Equal(t, []string{}, c.List)
	assert.Equal(t, map[string]string{}, c.Labels)
	c, err = config.Load[testConfig](context.Background(), d, env, reader("server: {port: 42}\ndebug: false\nname: file"))
	require.NoError(t, err)
	assert.Equal(t, 42, c.Server.Port)
	assert.Equal(t, "file", c.Name)
	assert.Equal(t, "localhost", c.Server.Host)
}

func TestFinalErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, input string
		kind        config.IssueKind
	}{
		{"required", "{}", config.IssueRequired},
		{"type", "server: {port: wrong}\ndebug: false\nname: ''", config.IssueType},
		{"quoted number", "server: {port: '12'}\ndebug: false\nname: ''", config.IssueType},
		{"null", "server: {port: null}\ndebug: false\nname: ''", config.IssueType},
		{"duration", "server: {port: 0, timeout: 2d}\ndebug: false\nname: ''", config.IssueType},
		{"unknown", "server: {prot: 1}", config.IssueUnknownField},
		{"duplicate", "server: {port: 1, port: 2}", config.IssueDuplicate},
		{"syntax", "server: [", config.IssueSyntax},
		{"multiple", "{}\n---\n{}", config.IssueSyntax},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := config.Load[testConfig](context.Background(), descriptor(), reader(tt.input))
			assert.Nil(t, c, "partial config")
			issue(t, err, tt.kind)
		})
	}
	_, err := config.Load[testConfig](context.Background(), descriptor())
	var e *config.Error
	require.ErrorAs(t, err, &e)
	require.Len(t, e.Issues, 3)
	assert.Equal(t, "server.port", e.Issues[0].Path)
	assert.Equal(t, "debug", e.Issues[1].Path)
	assert.Equal(t, "name", e.Issues[2].Path)
}

func TestSourceValidationBeforeMerge(t *testing.T) {
	t.Parallel()
	for name, format := range map[string]config.Format{"yaml": config.FormatYAML, "json": config.FormatJSON} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for name, input := range map[string]string{"unknown": `{"unknown": 1}`, "duplicate": `{"name":"a", "name":"b"}`} {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					s := config.Reader("low", strings.NewReader(input), format)
					_, err := config.Load[testConfig](context.Background(), descriptor(), s, environment(map[string]string{"APP_PORT": "0", "APP_DEBUG": "false", "EXPLICIT_NAME": ""}))
					require.Error(t, err, "source error hidden")
				})
			}
		})
	}
}

func TestFilesAndReaders(t *testing.T) {
	t.Parallel()
	for _, ext := range []string{"yaml", "yml", "json"} {
		t.Run(ext, func(t *testing.T) {
			t.Parallel()
			p := filepath.Join(t.TempDir(), "config."+ext)
			require.NoError(t, os.WriteFile(p, []byte(`{"server":{"port":1},"debug":false,"name":""}`), 0o600))
			c, err := config.Load[testConfig](context.Background(), descriptor(), config.File(p))
			require.NoError(t, err)
			assert.Equal(t, 1, c.Server.Port)
			require.NoError(t, os.WriteFile(p, []byte(`{"server":{"port":2},"debug":false,"name":""}`), 0o600))
			c, err = config.Load[testConfig](context.Background(), descriptor(), config.File(p))
			require.NoError(t, err)
			assert.Equal(t, 2, c.Server.Port)
		})
	}
	d := config.Descriptor{}
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	_, err := config.Load[struct{}](context.Background(), d, config.OptionalFile(missing))
	require.NoError(t, err)
	_, err = config.Load[struct{}](context.Background(), d, config.File(missing))
	require.ErrorIs(t, err, os.ErrNotExist)
	dir := filepath.Join(t.TempDir(), "dir.yaml")
	require.NoError(t, os.Mkdir(dir, 0o700))
	_, err = config.Load[struct{}](context.Background(), d, config.OptionalFile(dir))
	issue(t, err, config.IssueSource)
	_, err = config.Load[struct{}](context.Background(), d, config.Reader("broken", brokenReader{}, config.FormatYAML))
	issue(t, err, config.IssueSource)
	_, err = config.Load[struct{}](context.Background(), d, config.Reader("bad-format", strings.NewReader("{}"), 99))
	issue(t, err, config.IssueSource)
}

func TestPermissionError(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires POSIX permissions and an unprivileged user")
	}
	p := filepath.Join(t.TempDir(), "private.yaml")
	require.NoError(t, os.WriteFile(p, []byte("{}"), 0o000))
	for name, source := range map[string]config.Source{"required": config.File(p), "optional": config.OptionalFile(p)} {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load[struct{}](context.Background(), config.Descriptor{}, source)
			require.ErrorIs(t, err, os.ErrPermission)
		})
	}
}

func TestJSONErrorCategories(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		input string
		kind  config.IssueKind
	}{
		{`{"server":{"port":null},"debug":false,"name":""}`, config.IssueType},
		{`{"server":{"port":1,"port":2}}`, config.IssueDuplicate},
		{`{"unknown":1}`, config.IssueUnknownField},
		{`{"name":`, config.IssueSyntax},
	} {
		t.Run(string(tt.kind), func(t *testing.T) {
			t.Parallel()
			_, err := config.Load[testConfig](context.Background(), descriptor(), config.Reader("json", strings.NewReader(tt.input), config.FormatJSON))
			issue(t, err, tt.kind)
		})
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestEnvLoadTimeAndDisabled(t *testing.T) {
	d := descriptor()
	d.Fields[2].EnvDisabled = true
	d.Fields[2].HasDefault = true
	d.Fields[2].Default = "default"
	s := config.Env()
	t.Setenv("APP_PORT", "1")
	t.Setenv("APP_DEBUG", "false")
	t.Setenv("EXPLICIT_NAME", "ignored")
	for _, port := range []string{"2", "3"} {
		t.Setenv("APP_PORT", port)
		c, err := config.Load[testConfig](context.Background(), d, s)
		require.NoError(t, err)
		assert.Equal(t, port, fmt.Sprint(c.Server.Port))
		assert.Equal(t, "default", c.Name)
	}
}

func TestConstraintsAndRedaction(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		kind config.Kind
		raw  any
		c    config.Constraints
	}{
		{"minimum", config.KindInt, 0, config.Constraints{Min: 1}},
		{"maximum", config.KindInt, 10, config.Constraints{Max: 9}},
		{"enum", config.KindString, "bad", config.Constraints{Enum: []any{"good"}}},
		{"pattern", config.KindString, "sentinel-secret", config.Constraints{Pattern: "^safe$"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := config.Descriptor{Fields: []config.FieldDescriptor{{Path: "secret", Kind: tc.kind, GoIndex: []int{0}, HasDefault: true, Default: tc.raw, Secret: true, Constraints: tc.c}}}
			_, err := config.Load[struct{ X string }](context.Background(), d)
			issue(t, err, config.IssueConstraint)
			assert.NotContains(t, fmt.Sprintf("%+v %#v", err, err), "sentinel-secret")
		})
	}
	d := descriptor()
	d.Fields[0].Children[1].Secret = true
	_, err := config.Load[testConfig](context.Background(), d, environment(map[string]string{"APP_PORT": "sentinel-secret", "APP_DEBUG": "false", "EXPLICIT_NAME": ""}))
	issue(t, err, config.IssueType)
	assert.NotContains(t, fmt.Sprintf("%+v %#v", err, err), "sentinel-secret")
	assert.Contains(t, err.Error(), "env:APP_PORT")
	assert.Contains(t, err.Error(), "[REDACTED]")
}

func TestCancellation(t *testing.T) {
	t.Parallel()
	t.Run("before load", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		source := NewMockSource(t)
		c, err := config.Load[testConfig](ctx, descriptor(), source)
		assert.Nil(t, c)
		require.ErrorIs(t, err, context.Canceled)
		issue(t, err, config.IssueCanceled)
	})
	t.Run("during source", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		source := NewMockSource(t)
		source.EXPECT().Load(ctx, mock.Anything).RunAndReturn(func(context.Context, *config.Descriptor) (config.Document, error) {
			cancel()
			return config.Document{}, nil
		}).Once()
		c, err := config.Load[testConfig](ctx, descriptor(), source)
		assert.Nil(t, c)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestSyntheticPresence(t *testing.T) {
	t.Parallel()
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Path: "x", Kind: config.KindInt, GoIndex: []int{0}, HasDefault: true, Default: 42}}}
	source := NewMockSource(t)
	source.EXPECT().Load(context.Background(), &d).Return(config.Document{Values: map[string]config.RawValue{"x": {Value: 0, Present: false}}}, nil).Once()
	c, err := config.Load[struct{ X int }](context.Background(), d, source)
	require.NoError(t, err)
	assert.Equal(t, 42, c.X)
}

func TestConcurrentReader(t *testing.T) {
	t.Parallel()
	s := reader("server: {port: 0}\ndebug: false\nname: ''")
	d := descriptor()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			c, err := config.Load[testConfig](context.Background(), d, s)
			if assert.NoError(t, err) && assert.NotNil(t, c) {
				assert.Equal(t, "localhost", c.Server.Host)
			}
		})
	}
	wg.Wait()
}

func TestUnknownIgnoreAndNullOverride(t *testing.T) {
	t.Parallel()
	d := descriptor()
	d.UnknownFields = config.UnknownFieldsIgnore
	c, err := config.Load[testConfig](context.Background(), d, reader("unknown: x\nserver: {port: null, typo: 3}\ndebug: false\nname: ''"), environment(map[string]string{"APP_PORT": "5"}))
	require.NoError(t, err)
	assert.Equal(t, 5, c.Server.Port)
}

func TestListOfObjects(t *testing.T) {
	t.Parallel()
	type backend struct {
		Name string
		Port int
	}
	type cfg struct{ Backends []backend }
	item := config.FieldDescriptor{Kind: config.KindObject, Children: []config.FieldDescriptor{{Name: "name", Kind: config.KindString, Required: true, GoIndex: []int{0}}, {Name: "port", Kind: config.KindInt, HasDefault: true, Default: 80, GoIndex: []int{1}}}}
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Name: "backends", Path: "backends", Kind: config.KindList, GoIndex: []int{0}, EnvName: "BACKENDS", Item: &item}}}
	c, err := config.Load[cfg](context.Background(), d, environment(map[string]string{"BACKENDS": `[{"name":"api"}]`}))
	require.NoError(t, err)
	assert.Equal(t, []backend{{"api", 80}}, c.Backends)
	_, err = config.Load[cfg](context.Background(), d, reader("backends: [{typo: x}]"), environment(map[string]string{"BACKENDS": "[]"}))
	issue(t, err, config.IssueUnknownField)
}

func FuzzSources(f *testing.F) {
	f.Add([]byte(`{"server":{"port":0},"debug":false,"name":""}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		for _, format := range []config.Format{config.FormatJSON, config.FormatYAML} {
			_, _ = config.Load[testConfig](context.Background(), descriptor(), config.Reader("fuzz", strings.NewReader(string(b)), format))
		}
	})
}

func BenchmarkLoadReader(b *testing.B) {
	d := descriptor()
	s := reader("server: {port: 8080, timeout: 30s}\ndebug: false\nname: app\nlist: [a, b]\nlabels: {region: local}")
	b.ReportAllocs()
	for b.Loop() {
		if _, e := config.Load[testConfig](context.Background(), d, s); e != nil {
			b.Fatal(e)
		}
	}
}

func BenchmarkLoadEnv(b *testing.B) {
	d := descriptor()
	s := environment(map[string]string{"APP_PORT": "8080", "APP_TIMEOUT": "30s", "APP_DEBUG": "false", "EXPLICIT_NAME": "app"})
	b.ReportAllocs()
	for b.Loop() {
		if _, e := config.Load[testConfig](context.Background(), d, s); e != nil {
			b.Fatal(e)
		}
	}
}
