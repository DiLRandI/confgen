package config_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DiLRandI/confgen/config"
)

func TestJSONDuplicateKeyReportsDuplicateKeyLocation(t *testing.T) {
	input := "{\n  \"server\": {\n    \"port\": 1,\n    \"port\": 2\n  },\n  \"debug\": false,\n  \"name\": \"sentinel-secret\"\n}"
	_, err := config.Load[testConfig](context.Background(), descriptor(), config.Reader("json", strings.NewReader(input), config.FormatJSON))
	var ce *config.Error
	if !errors.As(err, &ce) || len(ce.Issues) != 1 {
		t.Fatalf("got %v, want one issue", err)
	}
	issue := ce.Issues[0]
	if issue.Kind != config.IssueDuplicate {
		t.Fatalf("got issue kind %q, want duplicate", issue.Kind)
	}
	if issue.Location == nil || issue.Location.Line != 4 || issue.Location.Column != 5 {
		t.Fatalf("got location %+v, want 4:5", issue.Location)
	}
	if strings.Contains(err.Error(), "sentinel-secret") {
		t.Fatalf("duplicate diagnostic leaked input value: %v", err)
	}
}
