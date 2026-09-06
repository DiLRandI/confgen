package infer_test

import (
	"github.com/DiLRandI/confgen/infer"
	"github.com/DiLRandI/confgen/input"
	"strings"
	"testing"
)

func TestInferenceInputLimits(t *testing.T) {
	data := []byte(`{"value":"secret-value"}`)
	for _, limit := range []input.Limit{input.Limit(len(data) - 1), input.Limit(len(data))} {
		model, err := infer.FromConfig("config.json", data, infer.Options{InputLimit: limit})
		if limit < input.Limit(len(data)) {
			if model != nil || err == nil || !strings.Contains(err.Error(), "byte limit") || strings.Contains(err.Error(), "secret-value") {
				t.Fatalf("oversize: %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}
func TestOverrideInputLimits(t *testing.T) {
	data := []byte("fields:\n  value: {type: string}\n")
	if _, err := infer.ParseOverridesWithLimit("overrides", data, input.Limit(len(data))); err != nil {
		t.Fatal(err)
	}
	if got, err := infer.ParseOverridesWithLimit("overrides", data, input.Limit(len(data)-1)); got != nil || err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("oversize: %v", err)
	}
	data = make([]byte, int(input.DefaultLimit)+1)
	if _, err := infer.ParseOverrides("overrides", data); err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("default overrides: %v", err)
	}
	if _, err := infer.FromConfig("input.json", data, infer.Options{}); err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("default inference: %v", err)
	}
}
