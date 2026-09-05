package config_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"go-config/config"
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
	if !errors.As(err, &e) || len(e.Issues) == 0 || e.Issues[0].Kind != kind {
		t.Fatalf("want %s, got %v", kind, err)
	}
}

func TestInvalidObjectShape(t *testing.T) {
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Name: "server", Kind: config.KindObject, GoIndex: []int{0}, Children: []config.FieldDescriptor{{Name: "host", Path: "server.host", Kind: config.KindString, GoIndex: []int{0}}}}}}
	c, err := config.Load[testConfig](context.Background(), d, reader("server: scalar"))
	if c != nil || err == nil {
		t.Fatalf("accepted scalar object: %+v, %v", c, err)
	}
	issue(t, err, config.IssueType)
}

func TestEmptyObjectNull(t *testing.T) {
	type cfg struct{ Empty struct{} }
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Name: "empty", Path: "empty", Kind: config.KindObject, GoIndex: []int{0}}}}
	if c, err := config.Load[cfg](context.Background(), d, reader("empty: null")); c != nil || err == nil {
		t.Fatalf("accepted null empty object: %+v %v", c, err)
	}
	if _, err := config.Load[cfg](context.Background(), d, reader("empty: null"), reader("empty: {}")); err != nil {
		t.Fatal(err)
	}
}

func TestPresencePrecedenceAndCollections(t *testing.T) {
	d := descriptor()
	low := reader("server: {host: original, port: wrong}\ndebug: true\nname: previous\nlist: [a, b]\nlabels: {a: one, b: two}")
	env := environment(map[string]string{"APP_PORT": "0", "APP_DEBUG": "false", "EXPLICIT_NAME": "", "APP_LIST": "[]", "APP_LABELS": "{}", "APP_TIMEOUT": "1h30m"})
	c, err := config.Load[testConfig](context.Background(), d, low, env)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Port != 0 || c.Server.Host != "original" || c.Server.Timeout != 90*time.Minute || c.Debug || c.Name != "" || len(c.List) != 0 || len(c.Labels) != 0 || c.List == nil || c.Labels == nil {
		t.Fatalf("%+v", c)
	}
	c, err = config.Load[testConfig](context.Background(), d, env, reader("server: {port: 42}\ndebug: false\nname: file"))
	if err != nil || c.Server.Port != 42 || c.Name != "file" || c.Server.Host != "localhost" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestFinalErrors(t *testing.T) {
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
			c, err := config.Load[testConfig](context.Background(), descriptor(), reader(tt.input))
			if c != nil {
				t.Fatal("partial config")
			}
			issue(t, err, tt.kind)
		})
	}
	_, err := config.Load[testConfig](context.Background(), descriptor())
	var e *config.Error
	if !errors.As(err, &e) || len(e.Issues) != 3 || e.Issues[0].Path != "server.port" || e.Issues[1].Path != "debug" || e.Issues[2].Path != "name" {
		t.Fatalf("%v", err)
	}
}

func TestSourceValidationBeforeMerge(t *testing.T) {
	for _, format := range []config.Format{config.FormatYAML, config.FormatJSON} {
		for _, input := range []string{`{"unknown": 1}`, `{"name":"a", "name":"b"}`} {
			s := config.Reader("low", strings.NewReader(input), format)
			_, e := config.Load[testConfig](context.Background(), descriptor(), s, environment(map[string]string{"APP_PORT": "0", "APP_DEBUG": "false", "EXPLICIT_NAME": ""}))
			if e == nil {
				t.Fatal("source error hidden")
			}
		}
	}
}

func TestFilesAndReaders(t *testing.T) {
	for _, ext := range []string{"yaml", "yml", "json"} {
		t.Run(ext, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config."+ext)
			if e := os.WriteFile(p, []byte(`{"server":{"port":1},"debug":false,"name":""}`), 0600); e != nil {
				t.Fatal(e)
			}
			c, e := config.Load[testConfig](context.Background(), descriptor(), config.File(p))
			if e != nil || c.Server.Port != 1 {
				t.Fatalf("%+v %v", c, e)
			}
			if e := os.WriteFile(p, []byte(`{"server":{"port":2},"debug":false,"name":""}`), 0600); e != nil {
				t.Fatal(e)
			}
			c, e = config.Load[testConfig](context.Background(), descriptor(), config.File(p))
			if e != nil || c.Server.Port != 2 {
				t.Fatalf("%+v %v", c, e)
			}
		})
	}
	d := config.Descriptor{}
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	if _, e := config.Load[struct{}](context.Background(), d, config.OptionalFile(missing)); e != nil {
		t.Fatal(e)
	}
	_, e := config.Load[struct{}](context.Background(), d, config.File(missing))
	if !errors.Is(e, os.ErrNotExist) {
		t.Fatal(e)
	}
	dir := filepath.Join(t.TempDir(), "dir.yaml")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	_, e = config.Load[struct{}](context.Background(), d, config.OptionalFile(dir))
	issue(t, e, config.IssueSource)
	_, e = config.Load[struct{}](context.Background(), d, config.Reader("broken", brokenReader{}, config.FormatYAML))
	issue(t, e, config.IssueSource)
	_, e = config.Load[struct{}](context.Background(), d, config.Reader("bad-format", strings.NewReader("{}"), 99))
	issue(t, e, config.IssueSource)
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
		c, e := config.Load[testConfig](context.Background(), d, s)
		if e != nil || fmt.Sprint(c.Server.Port) != port || c.Name != "default" {
			t.Fatalf("%+v %v", c, e)
		}
	}
}

func TestConstraintsAndRedaction(t *testing.T) {
	for _, tc := range []struct {
		kind config.Kind
		raw  any
		c    config.Constraints
	}{
		{config.KindInt, 0, config.Constraints{Min: 1}}, {config.KindInt, 10, config.Constraints{Max: 9}},
		{config.KindString, "bad", config.Constraints{Enum: []any{"good"}}}, {config.KindString, "sentinel-secret", config.Constraints{Pattern: "^safe$"}},
	} {
		d := config.Descriptor{Fields: []config.FieldDescriptor{{Path: "secret", Kind: tc.kind, GoIndex: []int{0}, HasDefault: true, Default: tc.raw, Secret: true, Constraints: tc.c}}}
		_, err := config.Load[struct{ X string }](context.Background(), d)
		issue(t, err, config.IssueConstraint)
		if strings.Contains(fmt.Sprintf("%+v %#v", err, err), "sentinel-secret") {
			t.Fatal("secret leaked")
		}
	}
	d := descriptor()
	d.Fields[0].Children[1].Secret = true
	_, err := config.Load[testConfig](context.Background(), d, environment(map[string]string{"APP_PORT": "sentinel-secret", "APP_DEBUG": "false", "EXPLICIT_NAME": ""}))
	issue(t, err, config.IssueType)
	if strings.Contains(fmt.Sprintf("%+v %#v", err, err), "sentinel-secret") || !strings.Contains(err.Error(), "env:APP_PORT") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatal(err)
	}
}

type sourceFunc struct {
	fn func(context.Context) (config.Document, error)
}

func (s sourceFunc) Name() string { return "custom" }
func (s sourceFunc) Load(ctx context.Context, _ *config.Descriptor) (config.Document, error) {
	return s.fn(ctx)
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c, e := config.Load[testConfig](ctx, descriptor())
	if c != nil || !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	issue(t, e, config.IssueCanceled)
	ctx, cancel = context.WithCancel(context.Background())
	s := sourceFunc{func(ctx context.Context) (config.Document, error) { cancel(); return config.Document{}, nil }}
	c, e = config.Load[testConfig](ctx, descriptor(), s)
	if c != nil || !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestSyntheticPresence(t *testing.T) {
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Path: "x", Kind: config.KindInt, GoIndex: []int{0}, HasDefault: true, Default: 42}}}
	s := sourceFunc{func(context.Context) (config.Document, error) {
		return config.Document{Values: map[string]config.RawValue{"x": {Value: 0, Present: false}}}, nil
	}}
	c, e := config.Load[struct{ X int }](context.Background(), d, s)
	if e != nil || c.X != 42 {
		t.Fatalf("%+v %v", c, e)
	}
}

func TestConcurrentReader(t *testing.T) {
	s := reader("server: {port: 0}\ndebug: false\nname: ''")
	d := descriptor()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			c, e := config.Load[testConfig](context.Background(), d, s)
			if e != nil || c.Server.Host != "localhost" {
				t.Errorf("%+v %v", c, e)
			}
		})
	}
	wg.Wait()
}

func TestUnknownIgnoreAndNullOverride(t *testing.T) {
	d := descriptor()
	d.UnknownFields = config.UnknownFieldsIgnore
	c, e := config.Load[testConfig](context.Background(), d, reader("unknown: x\nserver: {port: null, typo: 3}\ndebug: false\nname: ''"), environment(map[string]string{"APP_PORT": "5"}))
	if e != nil || c.Server.Port != 5 {
		t.Fatalf("%+v %v", c, e)
	}
}

func TestListOfObjects(t *testing.T) {
	type backend struct {
		Name string
		Port int
	}
	type cfg struct{ Backends []backend }
	item := config.FieldDescriptor{Kind: config.KindObject, Children: []config.FieldDescriptor{{Name: "name", Kind: config.KindString, Required: true, GoIndex: []int{0}}, {Name: "port", Kind: config.KindInt, HasDefault: true, Default: 80, GoIndex: []int{1}}}}
	d := config.Descriptor{Fields: []config.FieldDescriptor{{Name: "backends", Path: "backends", Kind: config.KindList, GoIndex: []int{0}, EnvName: "BACKENDS", Item: &item}}}
	c, e := config.Load[cfg](context.Background(), d, environment(map[string]string{"BACKENDS": `[{"name":"api"}]`}))
	if e != nil || !reflect.DeepEqual(c.Backends, []backend{{"api", 80}}) {
		t.Fatalf("%+v %v", c, e)
	}
	_, e = config.Load[cfg](context.Background(), d, reader("backends: [{typo: x}]"), environment(map[string]string{"BACKENDS": "[]"}))
	issue(t, e, config.IssueUnknownField)
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
