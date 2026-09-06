package infer

import (
	"strconv"
	"strings"
)

func canonicalName(key string) string {
	valid := len(key) > 0 && key[0] >= 'a' && key[0] <= 'z'
	for _, r := range key {
		valid = valid && (r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	}
	if valid {
		return key
	}
	runes := []rune(key)
	var out []rune
	separator := func() {
		if len(out) > 0 && out[len(out)-1] != '_' {
			out = append(out, '_')
		}
	}
	upper := func(r rune) bool { return r >= 'A' && r <= 'Z' }
	lower := func(r rune) bool { return r >= 'a' && r <= 'z' }
	digit := func(r rune) bool { return r >= '0' && r <= '9' }
	for i, r := range runes {
		switch {
		case upper(r):
			if i > 0 && (lower(runes[i-1]) || digit(runes[i-1]) || upper(runes[i-1]) && i+1 < len(runes) && lower(runes[i+1])) {
				separator()
			}
			out = append(out, r+'a'-'A')
		case lower(r) || digit(r):
			out = append(out, r)
		case r > 127:
			separator()
			out = append(out, []rune("u"+strconv.FormatInt(int64(r), 16))...)
			separator()
		default:
			separator()
		}
	}
	name := strings.Trim(string(out), "_")
	if name == "" {
		return "field"
	}
	if digit(rune(name[0])) {
		return "field_" + name
	}
	return name
}
