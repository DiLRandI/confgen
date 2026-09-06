package config_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/input"
)

func TestSourceInputLimits(t *testing.T) {
	data := `{"server":{"port":8080},"debug":false,"name":"example"}`
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	for _, limit := range []input.Limit{input.Limit(len(data) - 1), input.Limit(len(data)), input.Limit(len(data) + 1)} {
		for _, source := range []config.Source{config.ReaderWithLimit("test", strings.NewReader(data), config.FormatJSON, limit), config.FileWithLimit(path, limit), config.OptionalFileWithLimit(path, limit)} {
			got, err := config.Load[testConfig](context.Background(), descriptor(), source)
			if limit < input.Limit(len(data)) {
				var ce *config.Error
				var size *input.SizeError
				if got != nil || !errors.As(err, &ce) || ce.Issues[0].Kind != config.IssueSource || !errors.As(err, &size) {
					t.Fatalf("oversize: %+v %v", got, err)
				}
				if strings.Contains(err.Error(), "8080") {
					t.Fatalf("leaked value: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestEnvInputLimit(t *testing.T) {
	source := config.Env(config.WithEnvInputLimit(3), config.WithLookupEnv(func(string) (string, bool) { return "secret-value", true }))
	got, err := config.Load[testConfig](context.Background(), descriptor(), source)
	var size *input.SizeError
	if got != nil || !errors.As(err, &size) || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("env: %+v %v", got, err)
	}
}

func TestReaderDefaultLimit(t *testing.T) {
	data := strings.Repeat("x", int(input.DefaultLimit)+1)
	got, err := config.Load[testConfig](context.Background(), descriptor(), config.Reader("test", strings.NewReader(data), config.FormatYAML))
	var size *input.SizeError
	if got != nil || !errors.As(err, &size) || size.Limit != input.DefaultLimit {
		t.Fatalf("default reader: %v", err)
	}
}

func TestYAMLRejectedFeaturesRemainRejected(t *testing.T) {
	for _, data := range []string{"server: &server {port: 8080}\ncopy: *server\n", "server: {<<: {port: 8080}}\n"} {
		got, err := config.Load[testConfig](context.Background(), descriptor(), config.Reader("test", strings.NewReader(data), config.FormatYAML))
		var ce *config.Error
		if got != nil || !errors.As(err, &ce) || ce.Issues[0].Kind != config.IssueSyntax {
			t.Fatalf("YAML feature accepted: %v", err)
		}
	}
}
