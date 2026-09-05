// Package config loads generated, typed configuration from YAML, JSON, and
// environment variables. Applications normally call the generated package's
// Load, LoadContext, or MustLoad helpers with sources from this package.
//
// # Precedence and presence
//
// Sources run left-to-right; later sources override earlier ones. Schema defaults
// have the lowest priority. Required fields must be present, so false, zero, an
// empty string, and empty collections all satisfy requiredness. Only effective
// values are converted, so a later valid value can replace an earlier invalid
// value or null. Syntax errors, duplicate keys, and unknown file fields cannot
// be hidden by later sources.
//
// Objects merge by leaf path. Lists and string-keyed maps replace atomically.
// File values are strongly typed. Env scalars use Go's strconv and duration
// parsers without trimming whitespace. Env collections use JSON. Effective null
// values and non-finite floats are unsupported. String length counts Unicode
// code points, not bytes.
//
// # Sources and concurrency
//
// File and Env read their backing inputs at load time. Reader snapshots bytes
// at construction and can be reused concurrently. There is no mutable global
// registry. Generated descriptors must remain immutable, and custom sources
// shared between goroutines must themselves be concurrency-safe. Collections
// in returned configurations are newly allocated for each load.
//
// LoadContext propagates cancellation to sources and checks it before returning.
// Local filesystem reads are not forcibly interrupted while blocked. No partial
// configuration is returned on failure.
//
// # Diagnostics and secrets
//
// Use errors.As to inspect *Error and its ordered Issues. Context and filesystem
// errors support errors.Is. Library diagnostics never include raw field values;
// secret conversion and constraint issues also carry a [REDACTED] marker.
// Secrets remain ordinary Go fields: application logging or marshaling of the
// returned struct can expose them. Schema defaults, including secret defaults,
// are compiled into generated source; never put real credentials in a schema.
//
// YAML aliases and merge keys are unsupported. Unknown fields are errors unless
// the schema selects ignore. Sources accept exactly one YAML or JSON document.
package config
