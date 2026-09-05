package value

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestScalarConversion(t *testing.T) {
	cases := []struct {
		kind  Kind
		raw   any
		text  bool
		want  any
		valid bool
	}{
		{String, "", false, "", true},
		{Path, " ./data ", true, " ./data ", true},
		{Bool, false, false, false, true},
		{Bool, "FALSE", true, false, true},
		{Bool, "1", true, true, true},
		{Bool, "false", false, nil, false},
		{Int, 0, false, 0, true},
		{Int, "0", true, 0, true},
		{Int, " 1", true, nil, false},
		{Int, "1", false, nil, false},
		{Int, 1.0, false, nil, false},
		{Int, json.Number("1.0"), false, nil, false},
		{Int64, int64(math.MinInt64), false, int64(math.MinInt64), true},
		{Int64, "9223372036854775808", true, nil, false},
		{Int64, "-9223372036854775809", true, nil, false},
		{Uint, uint(0), false, uint(0), true},
		{Uint, "-1", true, nil, false},
		{Uint64, uint64(math.MaxUint64), false, uint64(math.MaxUint64), true},
		{Uint64, "18446744073709551616", true, nil, false},
		{Float64, 1.5, false, 1.5, true},
		{Float64, json.Number("1e2"), false, float64(100), true},
		{Float64, "NaN", true, nil, false},
		{Float64, "Inf", true, nil, false},
		{Float64, math.Inf(-1), false, nil, false},
		{Float64, "1e999", true, nil, false},
		{Duration, "1h30m", false, 90 * time.Minute, true},
		{Duration, "2d", true, nil, false},
		{Duration, 30, false, nil, false},
		{Duration, "2562048h", true, nil, false},
		{String, nil, false, nil, false},
		{String, true, false, nil, false},
	}
	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got, issues := Convert(Field{Kind: c.kind}, c.raw, c.text, "field")
			if (len(issues) == 0) != c.valid || c.valid && !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %#v %v, want %#v valid=%v", got, issues, c.want, c.valid)
			}
		})
	}
	if strconv.IntSize == 32 {
		if _, issues := Convert(Field{Kind: Int}, "2147483648", true, "x"); len(issues) == 0 {
			t.Fatal("platform int overflow accepted")
		}
		if _, issues := Convert(Field{Kind: Uint}, "4294967296", true, "x"); len(issues) == 0 {
			t.Fatal("platform uint overflow accepted")
		}
	}
}

func TestExactBoundsAndLengths(t *testing.T) {
	for _, tc := range []struct {
		f     Field
		raw   any
		valid bool
	}{
		{Field{Kind: Uint64, Constraints: Constraints{Min: uint64(math.MaxUint64)}}, uint64(math.MaxUint64 - 1), false},
		{Field{Kind: Uint64, Constraints: Constraints{Min: uint64(math.MaxUint64)}}, uint64(math.MaxUint64), true},
		{Field{Kind: String, Constraints: Constraints{MinLength: new(2), MaxLength: new(2)}}, "é世", true},
		{Field{Kind: String, Constraints: Constraints{MinLength: new(2)}}, "é", false},
		{Field{Kind: List, Item: &Field{Kind: String}, Constraints: Constraints{MaxLength: new(1)}}, []any{"a", "b"}, false},
		{Field{Kind: Map, MapValue: &Field{Kind: String}, Constraints: Constraints{MinLength: new(1)}}, map[string]any{}, false},
	} {
		_, issues := Convert(tc.f, tc.raw, false, "field")
		if (len(issues) == 0) != tc.valid {
			t.Fatalf("%+v: %v", tc.f, issues)
		}
	}
}

func FuzzScalarText(f *testing.F) {
	f.Add("0")
	f.Add("NaN")
	f.Add("1h30m")
	f.Fuzz(func(t *testing.T, s string) {
		for _, kind := range []Kind{String, Path, Bool, Int, Int64, Uint, Uint64, Float64, Duration} {
			_, _ = Convert(Field{Kind: kind}, s, true, "x")
		}
	})
}
