package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/DiLRandI/confgen/config"
	"github.com/DiLRandI/confgen/schema"
	"go.yaml.in/yaml/v3"
)

// ExampleYAML generates a schema-ordered configuration template. Secrets use
// neutral placeholders, including secrets nested inside collection defaults.
// Required placeholders can need editing to satisfy constraints.
func ExampleYAML(m *schema.Model) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("validated schema is nil")
	}
	var object func([]config.FieldDescriptor) *yaml.Node
	object = func(fields []config.FieldDescriptor) *yaml.Node {
		n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, f := range fields {
			k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: f.ExternalKey(), HeadComment: m.Descriptions[f.Path]}
			var v *yaml.Node
			if f.Kind == config.KindObject {
				v = object(f.Children)
			} else {
				v = &yaml.Node{}
				_ = v.Encode(exampleValue(f, nil, false))
			}
			n.Content = append(n.Content, k, v)
		}
		return n
	}
	var b bytes.Buffer
	e := yaml.NewEncoder(&b)
	e.SetIndent(2)
	if err := e.Encode(object(m.Descriptor.Fields)); err != nil {
		return nil, err
	}
	if err := e.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// ExampleEnv emits enabled environment mappings in schema order. Collections
// use JSON. These are documentation examples, not an executable shell script.
func ExampleEnv(m *schema.Model) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("validated schema is nil")
	}
	var b bytes.Buffer
	visit(m.Descriptor.Fields, func(f config.FieldDescriptor) {
		if f.Kind == config.KindObject || f.EnvDisabled || f.EnvName == "" {
			return
		}
		v := exampleValue(f, nil, false)
		var s string
		if f.Secret {
			s = ""
		} else if f.Kind == config.KindMap || f.Kind == config.KindList {
			data, _ := json.Marshal(v)
			s = string(data)
		} else {
			s = fmt.Sprint(v)
			if strings.ContainsAny(s, "\r\n\t") {
				s = strconv.Quote(s)
			}
		}
		fmt.Fprintf(&b, "%s=%s\n", f.EnvName, s)
	})
	return b.Bytes(), nil
}

func exampleValue(f config.FieldDescriptor, raw any, present bool) any {
	if !present && f.HasDefault {
		raw, present = f.Default, true
	}
	if f.Secret {
		present = false
		raw = nil
	}
	switch f.Kind {
	case config.KindObject:
		m, _ := raw.(map[string]any)
		out := map[string]any{}
		for _, c := range f.Children {
			v, ok := m[c.ExternalKey()]
			out[c.ExternalKey()] = exampleValue(c, v, ok)
		}
		return out
	case config.KindList:
		out := []any{}
		if present {
			for _, item := range raw.([]any) {
				out = append(out, exampleValue(*f.Item, item, true))
			}
		}
		return out
	case config.KindMap:
		out := map[string]any{}
		if present {
			for k, v := range raw.(map[string]any) {
				out[k] = exampleValue(*f.MapValue, v, true)
			}
		}
		return out
	default:
		if present {
			return raw
		}
		switch f.Kind {
		case config.KindString, config.KindPath:
			return ""
		case config.KindBool:
			return false
		case config.KindDuration:
			return "0s"
		default:
			return 0
		}
	}
}
