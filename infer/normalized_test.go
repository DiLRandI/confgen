package infer

import (
	"testing"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/internal/document"
	"github.com/stretchr/testify/require"
)

func TestNormalizedTextAdapter(t *testing.T) {
	t.Parallel()
	n := &document.Node{Fields: map[string]*document.Node{"port": {Value: "8080"}, "debug": {Value: "true"}, "timeout": {Value: "30s"}}, Order: []string{"port", "debug", "timeout"}}
	m, err := fromNode("text-adapter", n, Options{Package: "app"})
	require.NoError(t, err)
	for _, f := range m.Descriptor.Fields {
		require.Equal(t, config.KindString, f.Kind, "text adapter value was guessed")
	}
}

func TestNormalizedArrayAdapter(t *testing.T) {
	t.Parallel()
	n := &document.Node{Fields: map[string]*document.Node{"names": {Items: []*document.Node{{Value: "a"}, {Value: "b"}}}}, Order: []string{"names"}}
	m, err := fromNode("array-adapter", n, Options{Package: "app"})
	require.NoError(t, err)
	f := m.Descriptor.Fields[0]
	require.Equal(t, config.KindList, f.Kind, "array adapter failed")
	require.Equal(t, config.KindString, f.Item.Kind, "array adapter failed")
}
