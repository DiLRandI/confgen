# Architecture

## 1. Purpose

This document defines the v1 architecture. Public semantics described here are product decisions. Internal implementation details may change as long as those semantics remain intact.

The system has two halves:

1. **Build-time code generation** — parse and validate a YAML schema, then generate Go types and immutable descriptor metadata.
2. **Runtime configuration loading** — load raw values from ordered sources, merge by canonical field path, convert only effective values, validate, and populate the generated Go type.

The runtime must never parse `config.schema.yaml`.

## 2. High-level flow

```text
config.schema.yaml
       │
       ▼
   configgen
       │
       ├──────────────► generated Go types
       │
       ├──────────────► generated descriptor metadata
       │
       └──────────────► optional example files

At runtime:

schema defaults
       │
       ▼
source #1 ──► normalized raw document
       │
       ▼
source #2 ──► normalized raw document
       │
       ▼
ordered merge by canonical field path
       │
       ▼
convert effective values
       │
       ▼
required + constraint validation
       │
       ▼
populate generated Config
       │
       ▼
*Config
```

## 3. Package responsibilities

### `config`

Public runtime package.

Responsibilities:

- Define `Source` and built-in source constructors.
- Define descriptor types consumed by generated code.
- Apply generated defaults.
- Merge source documents in user-specified order.
- Convert effective values to schema target types.
- Validate required fields and constraints.
- Return structured configuration errors.
- Populate the generated configuration value.

It must not:

- parse the schema language;
- generate Go code;
- reconstruct configuration semantics from struct tags;
- maintain global mutable registries.

### `schema`

Build-time schema package.

Responsibilities:

- Parse exactly one YAML schema document.
- Detect duplicate schema keys.
- Reject unknown schema properties.
- Validate root and field definitions.
- Generate canonical field paths.
- Generate deterministic Go names.
- Generate deterministic environment names.
- Validate defaults and constraints.
- Detect naming and env collisions.

### `generator`

Build-time code-generation package.

Responsibilities:

- Consume only a validated schema model.
- Generate Go structs and runtime descriptors.
- Generate `Load`, `LoadContext`, and `MustLoad` helpers.
- Generate optional example YAML and env files.
- Format Go source using `go/format`.
- Write outputs atomically.

### `cmd/configgen`

Thin CLI wrapper. It parses arguments, invokes schema/generator packages, reports errors, and exits non-zero on failure. No business rules should live only in the CLI.

## 4. Canonical field paths

Every leaf field has one canonical dotted path:

```text
server.host
server.port
server.timeout
database.dsn
storage_dir
debug
```

Canonical paths are used for merge identity, validation reporting, source provenance, generated descriptors, and automatic env-name generation.

Objects are containers. Their leaf values merge independently. Lists and maps are leaf values and replace atomically.

## 5. Descriptor model

Generated code provides an immutable runtime descriptor. Exact internal representation may vary, but conceptually it contains:

```go
type Descriptor struct {
    RootName      string
    EnvPrefix     string
    UnknownFields UnknownFieldPolicy
    Fields        []FieldDescriptor
}

type FieldDescriptor struct {
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
    Children    []FieldDescriptor
    Item        *FieldDescriptor
    MapValue    *FieldDescriptor
}
```

Generated metadata, not struct-tag discovery, is authoritative at runtime.

## 6. Runtime source contract

Target interface:

```go
type Source interface {
    Name() string
    Load(ctx context.Context, descriptor *Descriptor) (Document, error)
}
```

Built-ins:

```go
func File(path string) Source
func OptionalFile(path string) Source
func Reader(name string, r io.Reader, format Format) Source
func Env(options ...EnvOption) Source
```

A source produces raw values plus provenance. It does not perform final merge or final schema validation.

## 7. Raw document model

Internally a source result must preserve value presence and origin independently from the value itself.

Conceptually:

```go
type Document struct {
    Values map[string]RawValue
}

type RawValue struct {
    Value    any
    Present  bool
    Source   string
    Location *Location
}
```

Presence must not be inferred from zero values. These are valid present values:

```text
0
false
""
[]
{}
```

## 8. Defaults

Defaults are the implicit lowest-priority source:

```text
defaults < source 1 < source 2 < ... < source N
```

A default establishes presence and therefore satisfies `required`.

Invalid defaults must be rejected at generation time.

## 9. Merge algorithm

For each runtime call:

1. Seed effective state from generated defaults.
2. For each source from left to right:
   - load it;
   - require valid YAML/JSON syntax if applicable;
   - detect duplicate keys and unknown fields;
   - normalize known fields to canonical paths;
   - replace effective values for every path present in the source.
3. Convert only effective final values.
4. Validate required fields.
5. Validate constraints.
6. Populate the generated struct.

Example:

```yaml
server:
  host: localhost
  port: 8080
```

plus `MYAPP_SERVER_PORT=9000` produces host `localhost` and port `9000`. The whole object is not replaced.

Lists and maps replace as complete values.

## 10. Conversion after merge

Conversion is intentionally delayed until after precedence resolution.

A lower-priority type-invalid raw value may be ignored if a higher-priority source supplies a valid value for the same canonical field. However, lower-priority sources must still be syntactically valid and must not contain unknown or duplicate keys.

This rule must be covered by explicit tests because it differs from eager struct decoding.

## 11. Assignment strategy

Reflection is acceptable in v1 for assigning converted values into generated structs.

Requirements:

- no `unsafe`;
- use generated metadata rather than reverse-engineering struct tags;
- deterministic assignment;
- errors identify schema paths, not reflection implementation details.

A future version may generate direct assignment code if profiling justifies it.

## 12. Concurrency and state

No package-level mutable configuration registry.

Multiple generated configurations may load independently and concurrently. Generated descriptors are immutable. Built-in sources read their backing source at `Load` time, not constructor time.

`Env()` therefore observes environment changes made before a subsequent load call. `File()` rereads the file for each load call. This is not hot reload.

## 13. Context behavior

`LoadContext` propagates context to every source. Cancellation returns an error and never a partially populated configuration. Generated `Load` uses `context.Background()`.

## 14. Dependency policy

Prefer:

- JSON: standard library plus custom duplicate-key detection;
- YAML: `gopkg.in/yaml.v3`;
- duration: `time.ParseDuration`;
- regex: `regexp`;
- Go formatting: `go/format`.

Avoid provider-specific dependencies in the core module.

## 15. Security boundary

`secret: true` means library-owned diagnostics never emit the raw effective value. This covers conversion errors, validation errors, runtime debug output owned by the package, and any future dump API.

It cannot prevent application code from printing generated structs directly. Documentation must state this clearly.

## 16. Compatibility boundary

Public semantic contracts include:

- source precedence;
- presence semantics;
- merge behavior;
- supported schema grammar;
- conversion rules;
- unknown-field behavior;
- required/default behavior;
- redaction guarantees;
- structured error categories;
- generated helper signatures.

Descriptor layout, reflection strategy, parser implementation, and repository file layout are internal implementation choices.
