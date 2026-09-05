package value

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"
)

// Problem contains no raw input, so it is safe for secret fields.
type Problem struct{ Path, Kind, Message string }

// Convert converts and validates a value. Text applies only to scalar env input;
// collections must have been decoded as JSON before calling Convert.
func Convert(f Field, raw any, text bool, path string) (any, []Problem) {
	fail := func(kind, msg string) (any, []Problem) { return nil, []Problem{{path, kind, msg}} }
	if raw == nil {
		return fail("type", "null is unsupported")
	}
	var result any
	var err error
	s, isString := raw.(string)
	switch f.Kind {
	case String, Path:
		if !isString {
			return fail("type", "expected string")
		}
		result = s
	case Bool:
		if text && isString {
			result, err = strconv.ParseBool(s)
		} else if b, ok := raw.(bool); ok {
			result = b
		} else {
			err = fmt.Errorf("type")
		}
	case Duration:
		if !isString {
			return fail("type", "expected duration string")
		}
		result, err = time.ParseDuration(s)
	case Int, Int64, Uint, Uint64, Float64:
		var number string
		if text && isString {
			number = s
		} else {
			switch n := raw.(type) {
			case json.Number:
				number = string(n)
			case int:
				number = strconv.Itoa(n)
			case int64:
				number = strconv.FormatInt(n, 10)
			case uint:
				number = strconv.FormatUint(uint64(n), 10)
			case uint64:
				number = strconv.FormatUint(n, 10)
			case float64:
				if f.Kind != Float64 {
					return fail("type", "expected integer")
				}
				number = strconv.FormatFloat(n, 'g', -1, 64)
			default:
				return fail("type", "expected number")
			}
		}
		bits := 64
		if f.Kind == Int || f.Kind == Uint {
			bits = strconv.IntSize
		}
		switch f.Kind {
		case Int, Int64:
			var n int64
			n, err = strconv.ParseInt(number, 10, bits)
			if f.Kind == Int {
				result = int(n)
			} else {
				result = n
			}
		case Uint, Uint64:
			var n uint64
			n, err = strconv.ParseUint(number, 10, bits)
			if f.Kind == Uint {
				result = uint(n)
			} else {
				result = n
			}
		case Float64:
			var n float64
			n, err = strconv.ParseFloat(number, 64)
			if math.IsNaN(n) || math.IsInf(n, 0) {
				err = fmt.Errorf("non-finite")
			}
			result = n
		}
	case Object:
		m, ok := raw.(map[string]any)
		if !ok {
			return fail("type", "expected object")
		}
		out := make(map[string]any)
		var problems []Problem
		for _, child := range f.Children {
			v, present := m[child.Name]
			if !present && child.HasDefault {
				v, present = child.Default, true
			}
			cp := child.Name
			if path != "" {
				cp = path + "." + cp
			}
			if !present && child.Kind == Object {
				v, present = map[string]any{}, true
			}
			if !present {
				if child.Required {
					problems = append(problems, Problem{cp, "required", "required value is missing"})
				}
				continue
			}
			cv, issues := Convert(child, v, false, cp)
			problems = append(problems, issues...)
			out[child.Name] = cv
		}
		if len(problems) > 0 {
			return nil, problems
		}
		result = out
	case List:
		a, ok := raw.([]any)
		if !ok || f.Item == nil {
			return fail("type", "expected list with item descriptor")
		}
		out := make([]any, len(a))
		var problems []Problem
		for i, v := range a {
			cv, issues := Convert(*f.Item, v, false, fmt.Sprintf("%s[%d]", path, i))
			out[i] = cv
			problems = append(problems, issues...)
		}
		if len(problems) > 0 {
			return nil, problems
		}
		result = out
	case Map:
		m, ok := raw.(map[string]any)
		if !ok || f.MapValue == nil {
			return fail("type", "expected map with value descriptor")
		}
		out := make(map[string]any, len(m))
		var problems []Problem
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			cv, issues := Convert(*f.MapValue, m[k], false, path+"{}")
			out[k] = cv
			problems = append(problems, issues...)
		}
		if len(problems) > 0 {
			return nil, problems
		}
		result = out
	default:
		return fail("type", "unsupported descriptor kind")
	}
	if err != nil {
		return fail("type", "expected valid "+string(f.Kind))
	}
	c := f.Constraints
	for _, bound := range []struct {
		value   any
		minimum bool
	}{{c.Min, true}, {c.Max, false}} {
		if bound.value == nil {
			continue
		}
		bf := f
		bf.Constraints = Constraints{}
		bv, issues := Convert(bf, bound.value, false, path)
		if len(issues) > 0 {
			return fail("constraint", "invalid bound in descriptor")
		}
		cmp, ok := Compare(result, bv)
		if !ok {
			return fail("constraint", "bound requires a numeric value")
		}
		if bound.minimum && cmp < 0 {
			return fail("constraint", "value is below minimum")
		}
		if !bound.minimum && cmp > 0 {
			return fail("constraint", "value exceeds maximum")
		}
	}
	length := -1
	switch v := result.(type) {
	case string:
		length = utf8.RuneCountInString(v)
	case []any:
		length = len(v)
	case map[string]any:
		length = len(v)
	}
	if c.MinLength != nil && length < *c.MinLength {
		return fail("constraint", "value is shorter than minimum length")
	}
	if c.MaxLength != nil && length > *c.MaxLength {
		return fail("constraint", "value exceeds maximum length")
	}
	if len(c.Enum) > 0 {
		matched := false
		ef := f
		ef.Constraints = Constraints{}
		for _, item := range c.Enum {
			ev, issues := Convert(ef, item, false, path)
			if len(issues) == 0 && reflect.DeepEqual(result, ev) {
				matched = true
				break
			}
		}
		if !matched {
			return fail("constraint", "value is not in enum")
		}
	}
	if c.Pattern != "" {
		r, e := regexp.Compile(c.Pattern)
		if e != nil {
			return fail("constraint", "invalid pattern in descriptor")
		}
		sv, ok := result.(string)
		if !ok || !r.MatchString(sv) {
			return fail("constraint", "value does not match pattern")
		}
	}
	return result, nil
}

// Compare compares converted numbers without losing integer precision.
func Compare(a, b any) (int, bool) {
	toRat := func(v any) *big.Rat {
		if d, ok := v.(time.Duration); ok {
			v = int64(d)
		}
		switch v.(type) {
		case int, int64, uint, uint64, float64:
		default:
			return nil
		}
		r, _ := new(big.Rat).SetString(fmt.Sprint(v))
		return r
	}
	x, y := toRat(a), toRat(b)
	if x == nil || y == nil {
		return 0, false
	}
	return x.Cmp(y), true
}
