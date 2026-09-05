package config_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-config/config"
)

func BenchmarkLoadDefaults(b *testing.B) {
	d := descriptor()
	d.Fields[0].Children[1].HasDefault = true
	d.Fields[0].Children[1].Default = 8080
	d.Fields[1].HasDefault = true
	d.Fields[1].Default = false
	d.Fields[2].HasDefault = true
	d.Fields[2].Default = "app"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := config.Load[testConfig](context.Background(), d); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkLoadJSON(b *testing.B) {
	d := descriptor()
	s := config.Reader("bench", strings.NewReader(`{"server":{"port":8080,"timeout":"30s"},"debug":false,"name":"app","list":["a","b"],"labels":{"region":"local"}}`), config.FormatJSON)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := config.Load[testConfig](context.Background(), d, s); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkLoadFile(b *testing.B) {
	d := descriptor()
	path := filepath.Join(b.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server: {port: 8080, timeout: 30s}\ndebug: false\nname: app\nlist: [a, b]\nlabels: {region: local}"), 0600); err != nil {
		b.Fatal(err)
	}
	s := config.File(path)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := config.Load[testConfig](context.Background(), d, s); err != nil {
			b.Fatal(err)
		}
	}
}
