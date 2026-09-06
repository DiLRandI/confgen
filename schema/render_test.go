package schema_test

import (
	"os"
	"testing"

	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/schema"
	"github.com/stretchr/testify/require"
)

func TestRenderPreservesContract(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../generator/testdata/all.schema.yaml")
	require.NoError(t, err)
	m, err := schema.Compile("all", data)
	require.NoError(t, err)
	before, err := generator.Generate(m, generator.Options{})
	require.NoError(t, err)
	rendered, err := schema.Render(m)
	require.NoError(t, err)
	round, err := schema.Compile("rendered", rendered)
	require.NoError(t, err)
	after, err := generator.Generate(round, generator.Options{})
	require.NoError(t, err)
	require.Equal(t, before, after, "schema rendering changed generated contract")
}
