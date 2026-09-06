package config_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/input"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourceInputLimits(t *testing.T) {
	t.Parallel()
	data := `{"server":{"port":8080},"debug":false,"name":"example"}`
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
	for _, tc := range []struct {
		name     string
		limit    input.Limit
		oversize bool
	}{
		{"over budget", input.Limit(len(data) - 1), true},
		{"at budget", input.Limit(len(data)), false},
		{"under budget", input.Limit(len(data) + 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for name, source := range map[string]config.Source{
				"reader":        config.ReaderWithLimit("test", strings.NewReader(data), config.FormatJSON, tc.limit),
				"file":          config.FileWithLimit(path, tc.limit),
				"optional file": config.OptionalFileWithLimit(path, tc.limit),
			} {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					got, err := config.Load[testConfig](context.Background(), descriptor(), source)
					if tc.oversize {
						assert.Nil(t, got)
						issue(t, err, config.IssueSource)
						var size *input.SizeError
						require.ErrorAs(t, err, &size)
						assert.Equal(t, tc.limit, size.Limit)
						assert.NotContains(t, err.Error(), "8080")
						return
					}
					require.NoError(t, err)
					require.NotNil(t, got)
					assert.Equal(t, 8080, got.Server.Port)
				})
			}
		})
	}
}

func TestEnvInputLimit(t *testing.T) {
	t.Parallel()
	source := config.Env(config.WithEnvInputLimit(3), config.WithLookupEnv(func(string) (string, bool) { return "secret-value", true }))
	got, err := config.Load[testConfig](context.Background(), descriptor(), source)
	assert.Nil(t, got)
	var size *input.SizeError
	require.ErrorAs(t, err, &size)
	assert.NotContains(t, err.Error(), "secret-value")
}

func TestReaderDefaultLimit(t *testing.T) {
	data := strings.Repeat("x", int(input.DefaultLimit)+1)
	got, err := config.Load[testConfig](context.Background(), descriptor(), config.Reader("test", strings.NewReader(data), config.FormatYAML))
	assert.Nil(t, got)
	var size *input.SizeError
	require.ErrorAs(t, err, &size)
	assert.Equal(t, input.DefaultLimit, size.Limit)
}

func TestYAMLRejectedFeaturesRemainRejected(t *testing.T) {
	t.Parallel()
	for name, data := range map[string]string{
		"alias":     "server: &server {port: 8080}\ncopy: *server\n",
		"merge key": "server: {<<: {port: 8080}}\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := config.Load[testConfig](context.Background(), descriptor(), config.Reader("test", strings.NewReader(data), config.FormatYAML))
			assert.Nil(t, got)
			issue(t, err, config.IssueSyntax)
		})
	}
}
