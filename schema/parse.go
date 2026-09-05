package schema

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Diagnostic describes an invalid schema without echoing schema values.
type Diagnostic struct {
	Location Location
	Path     string
	Message  string
}

func (d *Diagnostic) Error() string {
	prefix := fmt.Sprintf("%s:%d:%d", d.Location.File, d.Location.Line, d.Location.Column)
	if d.Path != "" {
		prefix += ": field " + strconv.Quote(d.Path)
	}
	return prefix + ": " + d.Message
}

// Parse reads exactly one YAML document. It checks structure, duplicate keys,
// and known properties. Call semantic validation before generating code.
// Aliases and YAML merge keys are unsupported.
func Parse(filename string, data []byte) (*Schema, error) {
	p := parser{filename: filename}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return nil, p.fail(nil, "", "expected one valid YAML schema document")
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, p.fail(&extra, "", "expected exactly one YAML document")
	}
	if err := p.inspect(&doc, ""); err != nil {
		return nil, err
	}
	if len(doc.Content) != 1 {
		return nil, p.fail(&doc, "", "expected a schema mapping")
	}
	root := doc.Content[0]
	props, err := p.mapping(root, "")
	if err != nil {
		return nil, err
	}
	s := &Schema{Name: "Config", UnknownFields: "error", Location: p.location(root)}
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		switch key.Value {
		case "version":
			if value.Tag != "!!int" || value.Kind != yaml.ScalarNode {
				return nil, p.fail(value, "", "version must be an integer")
			}
			if err := value.Decode(&s.Version); err != nil {
				return nil, p.fail(value, "", "version must fit an integer")
			}
		case "package", "name", "env_prefix", "unknown_fields":
			v, err := p.string(value, "", key.Value)
			if err != nil {
				return nil, err
			}
			switch key.Value {
			case "package":
				s.Package = v
			case "name":
				s.Name = v
			case "env_prefix":
				s.EnvPrefix = v
			case "unknown_fields":
				s.UnknownFields = v
			}
		case "fields":
			s.Fields, err = p.fields(value, "")
			if err != nil {
				return nil, err
			}
		default:
			return nil, p.fail(key, "", "unknown root property "+strconv.Quote(key.Value))
		}
	}
	for _, name := range []string{"version", "package", "fields"} {
		if _, ok := props[name]; !ok {
			return nil, p.fail(root, "", "missing root property "+name)
		}
	}
	return s, nil
}

type parser struct{ filename string }

func (p parser) location(n *yaml.Node) Location {
	l := Location{File: p.filename, Line: 1, Column: 1}
	if n != nil && n.Line > 0 {
		l.Line, l.Column = n.Line, n.Column
	}
	return l
}

func (p parser) fail(n *yaml.Node, path, message string) error {
	return &Diagnostic{Location: p.location(n), Path: path, Message: message}
}

// inspect runs before decoding, including inside defaults and constraints.
func (p parser) inspect(n *yaml.Node, path string) error {
	if n.Kind == yaml.AliasNode {
		return p.fail(n, path, "YAML aliases are unsupported")
	}
	if n.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for i := 0; i < len(n.Content); i += 2 {
			key, value := n.Content[i], n.Content[i+1]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return p.fail(key, path, "mapping keys must be strings; YAML merge keys are unsupported")
			}
			if seen[key.Value] {
				return p.fail(key, path, "duplicate mapping key")
			}
			seen[key.Value] = true
			if err := p.inspect(value, path); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range n.Content {
		if err := p.inspect(child, path); err != nil {
			return err
		}
	}
	return nil
}

func (p parser) mapping(n *yaml.Node, path string) (map[string]*yaml.Node, error) {
	if n.Kind != yaml.MappingNode || n.Tag != "!!map" {
		return nil, p.fail(n, path, "expected a mapping")
	}
	result := make(map[string]*yaml.Node, len(n.Content)/2)
	for i := 0; i < len(n.Content); i += 2 {
		result[n.Content[i].Value] = n.Content[i+1]
	}
	return result, nil
}

func (p parser) string(n *yaml.Node, path, property string) (string, error) {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
		return "", p.fail(n, path, property+" must be a string")
	}
	return n.Value, nil
}

func (p parser) boolean(n *yaml.Node, path, property string) (bool, error) {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!bool" {
		return false, p.fail(n, path, property+" must be a boolean")
	}
	var b bool
	if err := n.Decode(&b); err != nil {
		return false, p.fail(n, path, property+" must be a boolean")
	}
	return b, nil
}

func (p parser) fields(n *yaml.Node, parent string) ([]*Field, error) {
	if _, err := p.mapping(n, parent); err != nil {
		return nil, err
	}
	fields := make([]*Field, 0, len(n.Content)/2)
	for i := 0; i < len(n.Content); i += 2 {
		key := n.Content[i]
		path := key.Value
		if parent != "" {
			path = parent + "." + path
		}
		field, err := p.field(n.Content[i+1], path)
		if err != nil {
			return nil, err
		}
		field.Name, field.Location = key.Value, p.location(key)
		fields = append(fields, field)
	}
	return fields, nil
}

func (p parser) field(n *yaml.Node, path string) (*Field, error) {
	if _, err := p.mapping(n, path); err != nil {
		return nil, err
	}
	f := &Field{Path: path, Location: p.location(n), Properties: make(map[string]Location)}
	for i := 0; i < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		f.Properties[key.Value] = p.location(key)
		var err error
		switch key.Value {
		case "type":
			f.Type, err = p.string(value, path, key.Value)
		case "go_name":
			f.GoName, err = p.string(value, path, key.Value)
		case "description":
			f.Description, err = p.string(value, path, key.Value)
		case "required":
			f.Required, err = p.boolean(value, path, key.Value)
		case "secret":
			f.Secret, err = p.boolean(value, path, key.Value)
		case "env":
			if value.Kind != yaml.ScalarNode || (value.Tag != "!!str" && !(value.Tag == "!!bool" && strings.EqualFold(value.Value, "false"))) {
				return nil, p.fail(value, path, "env must be a string or false")
			}
			f.Env = value
		case "default":
			f.Default = value
		case "enum":
			f.Enum = value
		case "min":
			f.Min = value
		case "max":
			f.Max = value
		case "min_length":
			f.MinLength = value
		case "max_length":
			f.MaxLength = value
		case "pattern":
			f.Pattern = value
		case "fields":
			f.Fields, err = p.fields(value, path)
		case "items":
			f.Items, err = p.field(value, path+"[]")
		case "values":
			f.Values, err = p.field(value, path+"{}")
		default:
			return nil, p.fail(key, path, "unknown field property "+strconv.Quote(key.Value))
		}
		if err != nil {
			return nil, err
		}
	}
	if !f.Has("type") {
		return nil, p.fail(n, path, "missing field property type")
	}
	return f, nil
}
