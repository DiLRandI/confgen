package infer

import (
	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/internal/document"
	"testing"
)

func TestNormalizedTextAdapter(t *testing.T) {
	n := &document.Node{Fields: map[string]*document.Node{"port": {Value: "8080"}, "debug": {Value: "true"}, "timeout": {Value: "30s"}}, Order: []string{"port", "debug", "timeout"}}
	m, err := fromNode("text-adapter", n, Options{Package: "app"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range m.Descriptor.Fields {
		if f.Kind != config.KindString {
			t.Fatal("text adapter value was guessed")
		}
	}
}

func TestNormalizedArrayAdapter(t *testing.T) {
	n := &document.Node{Fields: map[string]*document.Node{"names": {Items: []*document.Node{{Value: "a"}, {Value: "b"}}}}, Order: []string{"names"}}
	m, err := fromNode("array-adapter", n, Options{Package: "app"})
	if err != nil {
		t.Fatal(err)
	}
	f := m.Descriptor.Fields[0]
	if f.Kind != config.KindList || f.Item.Kind != config.KindString {
		t.Fatal("array adapter failed")
	}
}
