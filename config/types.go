package config

import (
	"context"

	"github.com/DiLRandI/confgen/internal/value"
)

// Kind identifies a schema type.
type Kind = value.Kind

// Supported field kinds.
const (
	KindString   = value.String
	KindBool     = value.Bool
	KindInt      = value.Int
	KindInt64    = value.Int64
	KindUint     = value.Uint
	KindUint64   = value.Uint64
	KindFloat64  = value.Float64
	KindDuration = value.Duration
	KindPath     = value.Path
	KindObject   = value.Object
	KindList     = value.List
	KindMap      = value.Map
)

// Constraints describes inclusive limits, scalar enums, and Go regular expressions.
type Constraints = value.Constraints

// FieldDescriptor provides authoritative field metadata generated from a schema.
// GoIndex is relative to the containing struct. Collection values use JSON-like
// []any and map[string]any representations before assignment.
type FieldDescriptor = value.Field

// UnknownFieldPolicy controls unknown fields in file and reader sources.
type UnknownFieldPolicy string

// Unknown fields are rejected by default; ignore must be explicitly selected.
const (
	UnknownFieldsError  UnknownFieldPolicy = "error"
	UnknownFieldsIgnore UnknownFieldPolicy = "ignore"
)

// Descriptor is immutable metadata shared by concurrent loads.
type Descriptor struct {
	RootName      string
	EnvPrefix     string
	UnknownFields UnknownFieldPolicy
	Fields        []FieldDescriptor
}

// Source returns raw canonical leaf values. Implementations must not mutate the
// descriptor. A Source used concurrently must support concurrent Load calls.
type Source interface {
	Name() string
	Load(context.Context, *Descriptor) (Document, error)
}

// Document contains canonical leaf paths; objects are flattened and collections
// remain atomic. Absent entries and entries with Present=false do not override.
type Document struct{ Values map[string]RawValue }

// RawValue preserves presence, encoding, and provenance independently of Value.
// Text marks environment scalars or JSON-encoded collections. Non-text values
// use string, bool, numeric Go scalars, json.Number, []any, or map[string]any.
type RawValue struct {
	Value    any
	Present  bool
	Text     bool
	Source   string
	Location *Location
}
