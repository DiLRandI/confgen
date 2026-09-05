// Package generator renders validated schema models as deterministic Go source
// and YAML/environment examples. Call schema.Compile before Generate. Output has
// named structs, JSON/YAML tags, private descriptor metadata, and documented
// Load, LoadContext, and MustLoad helpers. No timestamps or absolute input paths
// appear in generated source.
//
// ExampleYAML and ExampleEnv replace secrets with neutral placeholders. Generated
// Go code must retain defaults to implement the schema, so real credentials must
// never be stored as schema defaults. Examples may require editing to satisfy
// required fields and constraints.
//
// WriteFiles stages outputs before atomically replacing each file and rolls back
// replacements on ordinary errors. A multi-file update is not crash-atomic.
// The configgen command exposes this workflow for go:generate directives.
package generator
