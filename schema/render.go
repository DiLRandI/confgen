package schema

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/DiLRandI/confgen/config"
	"go.yaml.in/yaml/v3"
)

// Render writes a contract as editable schema YAML in field order. The model
// need not originate in YAML; Compile validates the rendered schema before use.
func Render(m *Model) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("schema model is nil")
	}
	root := mapping()
	put(root, "version", encoded(1))
	put(root, "package", encoded(m.Package))
	if m.Name != "" && m.Name != "Config" {
		put(root, "name", encoded(m.Name))
	}
	if m.Descriptor.EnvPrefix != "" {
		put(root, "env_prefix", encoded(m.Descriptor.EnvPrefix))
	}
	if m.Descriptor.UnknownFields == config.UnknownFieldsIgnore {
		put(root, "unknown_fields", encoded("ignore"))
	}
	put(root, "fields", renderFields(m, m.Descriptor.Fields))
	var b bytes.Buffer
	e := yaml.NewEncoder(&b)
	e.SetIndent(2)
	if err := e.Encode(root); err != nil {
		return nil, err
	}
	if err := e.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func mapping() *yaml.Node      { return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"} }
func encoded(v any) *yaml.Node { n := &yaml.Node{}; _ = n.Encode(v); return n }
func put(n *yaml.Node, key string, value *yaml.Node) {
	n.Content = append(n.Content, encoded(key), value)
}

func renderFields(m *Model, fields []config.FieldDescriptor) *yaml.Node {
	n := mapping()
	for _, f := range fields {
		put(n, f.Name, renderField(m, f))
	}
	return n
}

func renderField(m *Model, f config.FieldDescriptor) *yaml.Node {
	n := mapping()
	put(n, "type", encoded(string(f.Kind)))
	if f.Key != "" && f.Key != f.Name {
		put(n, "key", encoded(f.Key))
	}
	if f.GoName != "" && f.GoName != GoName(f.Name) {
		put(n, "go_name", encoded(f.GoName))
	}
	if desc := m.Descriptions[f.Path]; desc != "" {
		put(n, "description", encoded(desc))
	}
	if f.Required {
		put(n, "required", encoded(true))
	}
	if f.Secret {
		put(n, "secret", encoded(true))
	}
	if f.EnvDisabled && !strings.ContainsAny(f.Path, "[]{}") {
		put(n, "env", encoded(false))
	} else if f.EnvName != "" {
		auto := strings.ToUpper(strings.ReplaceAll(f.Path, ".", "_"))
		if m.Descriptor.EnvPrefix != "" {
			auto = m.Descriptor.EnvPrefix + "_" + auto
		}
		if auto != f.EnvName {
			put(n, "env", encoded(f.EnvName))
		}
	}
	if f.Kind == config.KindObject {
		put(n, "fields", renderFields(m, f.Children))
	}
	if f.Item != nil {
		put(n, "items", renderField(m, *f.Item))
	}
	if f.MapValue != nil {
		put(n, "values", renderField(m, *f.MapValue))
	}
	if f.HasDefault {
		put(n, "default", renderDefault(f, f.Default))
	}
	c := f.Constraints
	if c.Min != nil {
		put(n, "min", encoded(c.Min))
	}
	if c.Max != nil {
		put(n, "max", encoded(c.Max))
	}
	if c.MinLength != nil {
		put(n, "min_length", encoded(*c.MinLength))
	}
	if c.MaxLength != nil {
		put(n, "max_length", encoded(*c.MaxLength))
	}
	if len(c.Enum) > 0 {
		put(n, "enum", encoded(c.Enum))
	}
	if c.Pattern != "" {
		put(n, "pattern", encoded(c.Pattern))
	}
	return n
}

func renderDefault(f config.FieldDescriptor, v any) *yaml.Node {
	if f.Kind == config.KindObject {
		if raw, ok := v.(map[string]any); ok {
			n := mapping()
			for _, c := range f.Children {
				if x, ok := raw[c.ExternalKey()]; ok {
					put(n, c.ExternalKey(), renderDefault(c, x))
				}
			}
			return n
		}
	}
	if f.Kind == config.KindList && f.Item != nil {
		if raw, ok := v.([]any); ok {
			n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			for _, x := range raw {
				n.Content = append(n.Content, renderDefault(*f.Item, x))
			}
			return n
		}
	}
	return encoded(v)
}
