# Design decisions

## Module identity

The repository has no remote or owner-selected module path. Use `go-config`
as a local development module until the owner chooses a publication path.
Do not invent a GitHub organization. Update imports before publishing.

## Go version

The module targets Go 1.27.1, the latest stable release in the official Go
download listing checked on 2026-09-05, as requested by the owner. CI uses the
version declared in go.mod. Go's automatic toolchain selection downloads it
when the installed toolchain is older.

## Parsing boundary

The schema parser returns an ordered AST with YAML nodes for defaults and
constraint values. This preserves integer precision, explicit nulls, property
presence, and source positions. Parsing checks grammar; a later semantic
validation phase will produce the normalized model used by generation.

Schema aliases and YAML merge keys are rejected. They are outside the specified
grammar and would obscure field order and duplicate-property diagnostics.

## Runtime choices reserved for implementation

Follow the specification recommendations: reject non-finite floats, snapshot
Reader bytes at construction, keep generated descriptors package-private, and
name nested types from their owning paths. These are planned, not implemented.
