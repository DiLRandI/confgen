package schema_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/DiLRandI/confgen/generator"
	"github.com/DiLRandI/confgen/schema"
)

func TestRenderPreservesContract(t *testing.T) {
	data, err := os.ReadFile("../generator/testdata/all.schema.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := schema.Compile("all", data)
	if err != nil {
		t.Fatal(err)
	}
	before, err := generator.Generate(m, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := schema.Render(m)
	if err != nil {
		t.Fatal(err)
	}
	round, err := schema.Compile("rendered", rendered)
	if err != nil {
		t.Fatal(err)
	}
	after, err := generator.Generate(round, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("schema rendering changed generated contract")
	}
}
