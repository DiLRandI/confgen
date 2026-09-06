package schema

import "go.yaml.in/yaml/v3"

// Location identifies a position in the schema input. Lines and columns start at 1.
type Location struct {
	File   string
	Line   int
	Column int
}

// Schema is a parsed schema, not yet a semantically validated generation model.
type Schema struct {
	Version       int
	Package       string
	Name          string
	EnvPrefix     string
	UnknownFields string
	Fields        []*Field
	Location      Location
}

// Field preserves declaration order and property presence. Node-valued properties
// retain YAML types and precision until semantic validation.
type Field struct {
	Name        string
	Key         string
	Path        string
	Type        string
	GoName      string
	Description string
	Required    bool
	Secret      bool
	Env         *yaml.Node
	Default     *yaml.Node
	Enum        *yaml.Node
	Min         *yaml.Node
	Max         *yaml.Node
	MinLength   *yaml.Node
	MaxLength   *yaml.Node
	Pattern     *yaml.Node
	Fields      []*Field
	Items       *Field
	Values      *Field
	Location    Location
	Properties  map[string]Location
}

// Has reports whether a property was explicitly written, including false or null.
func (f *Field) Has(property string) bool {
	_, ok := f.Properties[property]
	return ok
}
