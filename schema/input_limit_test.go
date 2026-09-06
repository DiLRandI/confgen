package schema_test

import (
	"github.com/DiLRandI/confgen/input"
	"github.com/DiLRandI/confgen/schema"
	"strings"
	"testing"
)

func TestSchemaInputLimits(t *testing.T) {
	data := []byte("version: 1\npackage: example\nfields:\n  value: {type: string, default: secret-value}\n")
	for _, limit := range []input.Limit{input.Limit(len(data) - 1), input.Limit(len(data)), input.Limit(len(data) + 1)} {
		model, err := schema.CompileWithLimit("schema", data, limit)
		if limit < input.Limit(len(data)) {
			if model != nil || err == nil || !strings.Contains(err.Error(), "byte limit") || strings.Contains(err.Error(), "secret-value") {
				t.Fatalf("oversize: %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	data = make([]byte, int(input.DefaultLimit)+1)
	if parsed, err := schema.Parse("schema", data); parsed != nil || err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("default: %v", err)
	}
}
