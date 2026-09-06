package schema_test

import (
	"strconv"
	"testing"

	"github.com/DiLRandI/confgen/input"
	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaInputLimits(t *testing.T) {
	data := []byte("version: 1\npackage: example\nfields:\n  value: {type: string, default: secret-value}\n")
	for _, limit := range []input.Limit{input.Limit(len(data) - 1), input.Limit(len(data)), input.Limit(len(data) + 1)} {
		t.Run(strconv.FormatInt(int64(limit), 10), func(t *testing.T) {
			model, err := schema.CompileWithLimit("schema", data, limit)
			if limit < input.Limit(len(data)) {
				assert.Nil(t, model)
				require.ErrorContains(t, err, "byte limit")
				assert.NotContains(t, err.Error(), "secret-value")
			} else {
				require.NoError(t, err)
			}
		})
	}
	data = make([]byte, int(input.DefaultLimit)+1)
	parsed, err := schema.Parse("schema", data)
	assert.Nil(t, parsed)
	require.ErrorContains(t, err, "byte limit")
}
