// Package value shares descriptor semantics between schema validation and runtime.
package value

// Kind is a schema value type.
type Kind string

// Supported schema types.
const (
	String   Kind = "string"
	Bool     Kind = "bool"
	Int      Kind = "int"
	Int64    Kind = "int64"
	Uint     Kind = "uint"
	Uint64   Kind = "uint64"
	Float64  Kind = "float64"
	Duration Kind = "duration"
	Path     Kind = "path"
	Object   Kind = "object"
	List     Kind = "list"
	Map      Kind = "map"
)

// Constraints contains inclusive bounds and scalar membership checks.
// Bounds and enum entries use the field's file representation.
type Constraints struct {
	Min       any
	Max       any
	MinLength *int
	MaxLength *int
	Enum      []any
	Pattern   string
}

// Field describes a field without relying on struct tags. Treat descriptors and
// all referenced slices, maps, and defaults as immutable after construction.
type Field struct {
	Name string
	// Key is the external YAML/JSON name. Empty uses the canonical Name.
	Key         string
	Path        string
	GoName      string
	GoIndex     []int
	Kind        Kind
	Required    bool
	HasDefault  bool
	Default     any
	Secret      bool
	EnvName     string
	EnvDisabled bool
	Constraints Constraints
	Children    []Field
	Item        *Field
	MapValue    *Field
}

// ExternalKey returns the key used in JSON and YAML documents.
func (f Field) ExternalKey() string {
	if f.Key != "" {
		return f.Key
	}
	return f.Name
}
