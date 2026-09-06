// Package document parses configuration syntax without schema conversion.
package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Node holds a raw value and source positions before precedence resolution.
type Node struct {
	Value        any
	Fields       map[string]*Node
	Order        []string
	NumberText   string
	Items        []*Node
	Line, Column int
}

// Error is a sanitized syntax or duplicate-key failure.
type Error struct {
	Kind         string
	Line, Column int
}

func (e *Error) Error() string { return "invalid document: " + e.Kind }

// ParseConfig selects a parser by filename extension without reading a file.
func ParseConfig(name string, data []byte) (*Node, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".yaml", ".yml":
		return Parse(data, false)
	case ".json":
		return Parse(data, true)
	default:
		return nil, fmt.Errorf("unsupported config extension; use .yaml, .yml, or .json")
	}
}

// Parse reads a single YAML or JSON document and rejects duplicate mapping keys.
// YAML aliases, merge keys, and non-string keys are unsupported.
func Parse(data []byte, jsonFormat bool) (*Node, error) {
	if jsonFormat {
		positions := newPositions(data)
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		n, e := jsonNode(d, data, positions, 0)
		if e != nil {
			return nil, e
		}
		if _, e = d.Token(); e != io.EOF {
			return nil, &Error{Kind: "syntax", Line: 1, Column: 1}
		}
		return n, nil
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	var root, extra yaml.Node
	if e := d.Decode(&root); e != nil {
		return nil, &Error{Kind: "syntax", Line: 1, Column: 1}
	}
	if e := d.Decode(&extra); e != io.EOF {
		return nil, &Error{Kind: "syntax", Line: extra.Line, Column: extra.Column}
	}
	if len(root.Content) != 1 {
		return nil, &Error{Kind: "syntax", Line: 1, Column: 1}
	}
	return yamlNode(root.Content[0], 0)
}

func yamlNode(n *yaml.Node, depth int) (*Node, error) {
	fail := func(kind string) (*Node, error) { return nil, &Error{Kind: kind, Line: n.Line, Column: n.Column} }
	if depth > 256 {
		return fail("syntax")
	}
	out := &Node{Line: n.Line, Column: n.Column}
	switch n.Kind {
	case yaml.MappingNode:
		if n.Tag != "!!map" {
			return fail("syntax")
		}
		out.Fields = map[string]*Node{}
		raw := map[string]any{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || k.Tag != "!!str" {
				return fail("syntax")
			}
			if _, ok := out.Fields[k.Value]; ok {
				return nil, &Error{Kind: "duplicate", Line: k.Line, Column: k.Column}
			}
			v, e := yamlNode(n.Content[i+1], depth+1)
			if e != nil {
				return nil, e
			}
			out.Fields[k.Value] = v
			out.Order = append(out.Order, k.Value)
			raw[k.Value] = v.Value
		}
		out.Value = raw
	case yaml.SequenceNode:
		out.Items = make([]*Node, 0, len(n.Content))
		if n.Tag != "!!seq" {
			return fail("syntax")
		}
		raw := make([]any, 0, len(n.Content))
		for _, item := range n.Content {
			v, e := yamlNode(item, depth+1)
			if e != nil {
				return nil, e
			}
			out.Items = append(out.Items, v)
			raw = append(raw, v.Value)
		}
		out.Value = raw
	case yaml.ScalarNode:
		if n.Tag == "!!int" || n.Tag == "!!float" {
			out.NumberText = n.Value
		}
		switch n.Tag {
		case "!!str", "!!bool", "!!int", "!!float", "!!null", "!!timestamp":
			if e := n.Decode(&out.Value); e != nil {
				return fail("syntax")
			}
		default:
			return fail("syntax")
		}
	default:
		return fail("syntax")
	}
	return out, nil
}

func jsonNode(d *json.Decoder, data []byte, positions positions, depth int) (*Node, error) {
	line, col := positions.position(int(d.InputOffset()))
	fail := func(kind string) (*Node, error) { return nil, &Error{Kind: kind, Line: line, Column: col} }
	if depth > 256 {
		return fail("syntax")
	}
	t, e := d.Token()
	if e != nil {
		return fail("syntax")
	}
	out := &Node{Line: line, Column: col}
	if delim, ok := t.(json.Delim); ok {
		switch delim {
		case '{':
			out.Fields = map[string]*Node{}
			raw := map[string]any{}
			for d.More() {
				keyOffset := int(d.InputOffset())
				key, e := d.Token()
				if e != nil {
					return fail("syntax")
				}
				s, ok := key.(string)
				if !ok {
					return fail("syntax")
				}
				if _, ok := out.Fields[s]; ok {
					line, col := positions.position(jsonKeyOffset(data, keyOffset))
					return nil, &Error{Kind: "duplicate", Line: line, Column: col}
				}
				v, e := jsonNode(d, data, positions, depth+1)
				if e != nil {
					return nil, e
				}
				out.Fields[s] = v
				out.Order = append(out.Order, s)
				raw[s] = v.Value
			}
			if end, e := d.Token(); e != nil || end != json.Delim('}') {
				return fail("syntax")
			}
			out.Value = raw
		case '[':
			out.Items = []*Node{}
			raw := []any{}
			for d.More() {
				v, e := jsonNode(d, data, positions, depth+1)
				if e != nil {
					return nil, e
				}
				out.Items = append(out.Items, v)
				raw = append(raw, v.Value)
			}
			if end, e := d.Token(); e != nil || end != json.Delim(']') {
				return fail("syntax")
			}
			out.Value = raw
		default:
			return fail("syntax")
		}
	} else {
		out.Value = t
	}
	return out, nil
}

type positions struct {
	newlines []int
	length   int
}

func newPositions(data []byte) positions {
	p := positions{newlines: make([]int, 0, bytes.Count(data, []byte{'\n'})), length: len(data)}
	for i, b := range data {
		if b == '\n' {
			p.newlines = append(p.newlines, i)
		}
	}
	return p
}

func (p positions) position(offset int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > p.length {
		offset = p.length
	}
	line := sort.Search(len(p.newlines), func(i int) bool { return p.newlines[i] >= offset })
	column := offset + 1
	if line > 0 {
		column = offset - p.newlines[line-1]
	}
	return line + 1, column
}

func jsonKeyOffset(data []byte, start int) int {
	if start < 0 {
		start = 0
	}
	for start < len(data) {
		switch data[start] {
		case ' ', '\t', '\r', '\n', ',':
			start++
		default:
			return start
		}
	}
	return len(data)
}

// Keys returns deterministic mapping order for diagnostics.
func (n *Node) Keys() []string {
	keys := make([]string, 0, len(n.Fields))
	for k := range n.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// OrderedKeys preserves declaration order, with a sorted fallback for adapters.
func (n *Node) OrderedKeys() []string {
	if len(n.Order) == len(n.Fields) {
		return n.Order
	}
	return n.Keys()
}

// Scalar normalizes numeric representations for build-time inference without
// changing Value, whose original encoding is needed by runtime conversion.
func (n *Node) Scalar() (any, error) {
	switch x := n.Value.(type) {
	case int:
		return int64(x), nil
	case uint:
		return uint64(x), nil
	case json.Number:
		s := string(x)
		if !strings.ContainsAny(s, ".eE") {
			if v, err := strconv.ParseInt(s, 10, 64); err == nil {
				return v, nil
			}
			if v, err := strconv.ParseUint(s, 10, 64); err == nil {
				return v, nil
			}
			return nil, fmt.Errorf("integer is outside the supported 64-bit range")
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
			return nil, fmt.Errorf("float must be finite and fit float64")
		}
		return v, nil
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return nil, fmt.Errorf("float must be finite")
		}
		if n.NumberText != "" && !strings.ContainsAny(n.NumberText, ".eE") {
			return nil, fmt.Errorf("integer is outside the supported 64-bit range")
		}
		return x, nil
	default:
		return x, nil
	}
}

// String returns only structural information, never raw values.
func (n *Node) String() string { return fmt.Sprintf("document node at %d:%d", n.Line, n.Column) }
