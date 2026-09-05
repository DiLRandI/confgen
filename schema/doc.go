// Package schema compiles build-time YAML schemas into validated generation
// models. Compile combines Parse and Validate. Parse alone checks grammar and
// preserves property presence, declaration order, and source positions; Validate
// checks types, constraints, defaults, Go names, and environment collisions.
//
// Supported types are string, path, bool, int, int64, uint, uint64, float64,
// duration, object, list, and map. Lists contain scalars or objects; map values
// are scalar. Constraints include required, default, min/max, min_length and
// max_length, scalar enums, and Go regular expressions. Duration values use
// time.ParseDuration syntax. Required is presence-based and defaults establish
// presence. String length counts Unicode code points.
//
// The schema reference is in docs/schema.md. Runtime applications use generated
// descriptors and never parse schema YAML. Models and descriptors must be
// treated as immutable. Compile reports the first structural or semantic error
// as a *Diagnostic without echoing raw default values.
package schema
