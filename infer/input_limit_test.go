package infer_test

import (
	"strconv"
	"testing"

	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/input"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInferenceInputLimits(t *testing.T) {
	t.Parallel()
	data := []byte(`{"value":"secret-value"}`)
	for _, limit := range []input.Limit{input.Limit(len(data) - 1), input.Limit(len(data))} {
		t.Run(strconv.FormatInt(int64(limit), 10), func(t *testing.T) {
			t.Parallel()
			model, err := infer.FromConfig("config.json", data, infer.Options{InputLimit: limit})
			if limit < input.Limit(len(data)) {
				assert.Nil(t, model)
				require.ErrorContains(t, err, "byte limit")
				assert.NotContains(t, err.Error(), "secret-value")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOverrideInputLimits(t *testing.T) {
	data := []byte("fields:\n  value: {type: string}\n")
	_, err := infer.ParseOverridesWithLimit("overrides", data, input.Limit(len(data)))
	require.NoError(t, err)
	got, err := infer.ParseOverridesWithLimit("overrides", data, input.Limit(len(data)-1))
	assert.Nil(t, got)
	require.ErrorContains(t, err, "byte limit")
	data = make([]byte, int(input.DefaultLimit)+1)
	_, err = infer.ParseOverrides("overrides", data)
	require.ErrorContains(t, err, "byte limit")
	_, err = infer.FromConfig("input.json", data, infer.Options{})
	require.ErrorContains(t, err, "byte limit")
}
