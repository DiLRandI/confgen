package config_test

import (
	"context"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONDuplicateKeyReportsDuplicateKeyLocation(t *testing.T) {
	t.Parallel()
	input := "{\n  \"server\": {\n    \"port\": 1,\n    \"port\": 2\n  },\n  \"debug\": false,\n  \"name\": \"sentinel-secret\"\n}"
	_, err := config.Load[testConfig](context.Background(), descriptor(), config.Reader("json", strings.NewReader(input), config.FormatJSON))
	var ce *config.Error
	require.ErrorAs(t, err, &ce)
	require.Len(t, ce.Issues, 1)
	issue := ce.Issues[0]
	assert.Equal(t, config.IssueDuplicate, issue.Kind)
	require.NotNil(t, issue.Location)
	assert.Equal(t, 4, issue.Location.Line)
	assert.Equal(t, 5, issue.Location.Column)
	assert.NotContains(t, err.Error(), "sentinel-secret")
}
